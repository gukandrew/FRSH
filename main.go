package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
)

func main() {
	cfg := getConfig()

	compressAndCopyDirectories(cfg)
	copyDirectories(cfg)
}

func maxUint(x uint, y uint) uint {
	if x > y {
		return x
	}

	return y
}

func sshHostAlive(idx int, server Server, verbose uint) bool {
	connectStr := fmt.Sprintf("%v@%v", server.User, server.Host)

	args := []string{"-q", "-o BatchMode=yes", "-o StrictHostKeyChecking=no", "-o ConnectTimeout=5", "-i", server.PrivateKey, "-p", server.Port, connectStr, "exit 0"}

	cmd := ExecAndLog(verbose, "ssh", "", args, false, idx)
	code := cmd.ProcessState.ExitCode()

	return code == 0
}

func compressAndCopyDirectories(cfg *Config) {
	if len(cfg.CompressAndCopy) <= 0 {
		return
	}

	timestamp := strconv.Itoa(int(time.Now().Unix()))
	remoteMarker := regexp.MustCompile(`^remote:`)

	for index, syncItem := range cfg.CompressAndCopy {
		verbose := maxUint(cfg.Verbose, syncItem.Verbose)
		idx := index + 1

		server := cfg.Servers[syncItem.Server]

		if !sshHostAlive(idx, server, verbose) {
			fmt.Printf("[%v] Host '%v' is unreachable! \n", idx, syncItem.Server)
			continue
		}

		connectStr := fmt.Sprintf("%v@%v", server.User, server.Host)
		dest, source, fromRemote := cleanupSourceDest(cfg, syncItem.Source, syncItem.Dest, remoteMarker)
		excludes := parseExcludes(syncItem.Exclude)
		port := server.Port
		privateKey := server.PrivateKey

		filename := generateArchiveName(syncItem, timestamp)

		var args []string

		if fromRemote {
			tarArgs := []string{excludes, "-zcvf", filename, source}
			tarArgsStr := strings.Join(append([]string{"tar"}, tarArgs...), " ")

			args = []string{"-i", privateKey, "-p", port, connectStr, tarArgsStr}

			fmt.Printf("[Tar and Copy %v] Compressing remote directory '%v'. This could take long time depending on size! \n", idx, source)
			ExecAndLog(verbose, "ssh", "Tar and Copy", args, syncItem.DryRun, idx)

			source = fmt.Sprintf("%v:%v", connectStr, filename)
		} else {
			args = []string{excludes, "-zcvf", filename, source}

			fmt.Printf("[Tar and Copy %v] Compressing local directory '%v'. This could take long time depending on size! \n", idx, source)
			ExecAndLog(verbose, "tar", "Tar and Copy", args, syncItem.DryRun, idx)

			source = filename
			dest = fmt.Sprintf("%v:%v", connectStr, dest)
		}

		args = []string{"-i", privateKey, "-P", port, source, dest}

		LogAction("scp", "Tar and Copy", syncItem.Log, args, idx, source, dest)
		ExecAndLog(1, "scp", "Tar and Copy", args, syncItem.DryRun, idx)
	}
}

func generateArchiveName(syncItem CompressAndCopyItem, timestamp string) string {
	filename := "/dev/null"

	if !syncItem.DryRun {
		filename = "/tmp/" + syncItem.Filename + "_" + timestamp + ".tar.gz"
	}
	return filename
}

func copyDirectories(cfg *Config) {
	if len(cfg.Sync) <= 0 {
		return
	}

	remoteMarker := regexp.MustCompile(`^remote:`)

	for index, syncItem := range cfg.Sync {
		verbose := maxUint(cfg.Verbose, syncItem.Verbose)

		idx := index + 1

		server := cfg.Servers[syncItem.Server]

		if !sshHostAlive(idx, server, verbose) {
			fmt.Printf("[%v] Server '%v' Host is unreachable! \n", idx, syncItem.Server)
			continue
		}

		dest, source, fromRemote := cleanupSourceDest(cfg, syncItem.Source, syncItem.Dest, remoteMarker)
		source, dest = initializeSourceDest(server, syncItem, fromRemote, source, dest)

		sshArgs, args := generateRsyncArgs(cfg, syncItem, source, dest)

		LogAction("rsync", "", syncItem.Log, args, idx, source, dest)
		ExecuteRsyncWithProgress(verbose, idx, args, sshArgs, cfg, syncItem)
		ExecAndLog(verbose, "rsync", "", args, syncItem.DryRun, idx)
	}
}

func initializeSourceDest(server Server, syncItem SyncItem, fromRemote bool, source string, dest string) (string, string) {
	connectStr := fmt.Sprintf("%v@%v", server.User, server.Host)
	if fromRemote {
		source = fmt.Sprintf("%v:%v", connectStr, source)
	} else {
		dest = fmt.Sprintf("%v:%v", connectStr, dest)
	}
	return source, dest
}

func LogAction(execname string, details string, logMsg string, args []string, idx int, source string, dest string) {
	if details != "" {
		details = details + " "
	}

	if logMsg != "" {
		fmt.Printf(">> [%v%v] %v:\n", details, idx, logMsg)
	} else {
		fmt.Printf(">> [%v%v] Copying %v into %v:\n", details, idx, source, dest)
	}
}

func ExecAndLog(verbose uint, execname string, details string, args []string, dryRun bool, idx int) *exec.Cmd {
	if verbose == 1 {
		if details != "" {
			details = details + " "
		}

		fmt.Printf(">> [%v%v] RUN: %v\n", details, idx, strings.Join(append([]string{execname}, args...), " "))
	}

	cmd := exec.Command(execname, args...)

	if !dryRun {
		if verbose == 2 {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stdout
		}
		cmd.Run()
	}

	return cmd
}

func cleanupSourceDest(cfg *Config, in_source string, in_dest string, remoteMarker *regexp.Regexp) (string, string, bool) {
	dest := remoteMarker.ReplaceAllString(in_dest, "")
	source := remoteMarker.ReplaceAllString(in_source, "")
	fromRemote := remoteMarker.FindString(in_source) != ""

	return dest, source, fromRemote
}

func ExecuteRsyncWithProgress(verbose uint, idx int, args []string, sshArgs string, cfg *Config, syncItem SyncItem) {
	cmd := exec.Command("rsync", args...)

	// rsync-ssh crutch: no other way to use ssh for rsync here in golang :(
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RSYNC_RSH=\""+sshArgs+"\"")
	// end rsync-ssh crutch

	total := executeRsyncPrediction(args, sshArgs)
	bar := createProgressbar(total, "syncing")

	cmdWriter := &progressWriter{progressBar: bar}
	mw := io.MultiWriter(cmdWriter)
	if verbose == 2 {
		mw = io.MultiWriter(cmdWriter, os.Stdout)
	}

	cmd.Stdout = mw
	cmd.Stderr = mw

	cmd.Run()
	bar.Finish()
}

func generateRsyncArgs(cfg *Config, syncItem SyncItem, source string, dest string) (string, []string) {
	args := []string{"-avz", "--progress", "--out-format=%l###%n"}

	if syncItem.DryRun {
		args = append(args, "--dry-run")
	}

	if syncItem.DeleteExtraneousFromDest {
		args = append(args, "--delete")
	}

	excludes := parseExcludes(syncItem.Exclude)
	if excludes != "" {
		args = append(args, excludes)
	}

	args = append(args, source, dest)

	port := cfg.Servers[syncItem.Server].Port
	privateKey := cfg.Servers[syncItem.Server].PrivateKey
	sshArgs := "\"" + strings.Join([]string{"ssh", "-i", privateKey, "-p", port}, " ") + "\""

	return sshArgs, args
}

func createProgressbar(total int, description string) *progressbar.ProgressBar {
	bar := progressbar.NewOptions64(
		int64(total),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Printf("\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetPredictTime(false),
	)
	bar.RenderBlank()
	return bar
}

func executeRsyncPrediction(args []string, sshArgs string) int {
	prediction := exec.Command("rsync", append([]string{"--dry-run"}, args...)...)
	prediction.Env = os.Environ()
	prediction.Env = append(prediction.Env, "RSYNC_RSH=\""+sshArgs+"\"")

	total := 0
	prediction.Stdout = &progressWriter{total: &total, progressBar: nil}
	prediction.Run()

	if total == 0 {
		total = 100
	}

	return total
}

func parseExcludes(v []string) string {
	excludes := []string{}

	for _, exclude := range v {
		excludes = append(excludes, fmt.Sprintf("--exclude=\"%v\"", exclude))
	}
	return strings.Join(excludes, " ")
}
