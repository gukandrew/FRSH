package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Verbose bool `yaml:"verbose"`

	Servers map[string]struct {
		User       string `yaml:"user"`
		Host       string `yaml:"host"`
		PrivateKey string `yaml:"private_key"`
		Port       string `yaml:"port"`
	} `yaml:"servers"`

	CompressAndCopy []struct {
		Server   string `yaml:"server"`
		Filename string `yaml:"filename"`
		Log      string `yaml:"log"`
		Source   string `yaml:"source"`
		Dest     string `yaml:"dest"`
		Verbose  bool   `yaml:"verbose"`
		DryRun   bool   `yaml:"dry_run"`
		Exclude  []string
	} `yaml:"compress_and_copy"`

	Sync []struct {
		Server                   string `yaml:"server"`
		Log                      string `yaml:"log"`
		Source                   string `yaml:"source"`
		Dest                     string `yaml:"dest"`
		DeleteExtraneousFromDest bool   `yaml:"delete_extraneous_from_dest"`
		Verbose                  bool   `yaml:"verbose"`
		DryRun                   bool   `yaml:"dry_run"`
		Exclude                  []string
	} `yaml:"sync"`
}

func main() {
	cfgPath, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := NewConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	compressAndCopyDirectories(cfg)
	copyDirectories(cfg)
}

func compressAndCopyDirectories(cfg *Config) {
	if len(cfg.CompressAndCopy) <= 0 {
		return
	}

	timestamp := strconv.Itoa(int(time.Now().Unix()))
	remoteMarker := regexp.MustCompile(`^remote:`)

	for index, v := range cfg.CompressAndCopy {
		idx := index + 1

		server := cfg.Servers[v.Server]
		connectStr := fmt.Sprintf("%v@%v", server.User, server.Host)

		excludes := parseExcludes(v.Exclude)
		// matcher := regexp.MustCompile(`\"`)
		// excludes = matcher.ReplaceAllString(excludes, "\\\"")

		port := server.Port
		privateKey := server.PrivateKey

		fromRemote := remoteMarker.FindString(v.Source) != ""
		source := remoteMarker.ReplaceAllString(v.Source, "")
		dest := remoteMarker.ReplaceAllString(v.Dest, "")
		filename := v.Filename + "_" + timestamp + ".tar.gz"

		if fromRemote {
			if v.Log != "" {
				fmt.Printf(">> [Tar and Copy %v] %v:\n", idx, v.Log)
			} else {
				fmt.Printf(">> [Tar and Copy %v] Copying %v into %v:\n", idx, fmt.Sprintf("%v:%v", connectStr, source), dest)
			}

			if v.DryRun {
				filename = "/dev/null"
			} else {
				filename = "/tmp/" + filename
			}

			tarArgs := []string{excludes, "-zcvf", filename, source}
			tarArgsStr := strings.Join(append([]string{"tar"}, tarArgs...), " ")

			args := []string{"-i", privateKey, "-p", port, connectStr, tarArgsStr}

			fmt.Printf(">> [%v] RUN: %v\n", idx, strings.Join(append([]string{"ssh"}, args...), " "))

			cmd := exec.Command("ssh", args...)

			var outb, errb bytes.Buffer
			cmd.Stdout = &outb
			cmd.Stderr = &errb
			cmd.Run()

			if cfg.Verbose || v.Verbose {
				fmt.Println(outb.String())
				fmt.Println(errb.String())
			}

			// Downloading using SCP
			source = fmt.Sprintf("%v:%v", connectStr, filename)
			args = []string{"-i", privateKey, "-P", port, source, dest}

			fmt.Printf(">> [%v] RUN: %v\n", idx, strings.Join(append([]string{"scp"}, args...), " "))

			cmd = exec.Command("scp", args...)

			if !v.DryRun {
				var outb, errb bytes.Buffer
				cmd.Stdout = &outb
				cmd.Stderr = &errb
				cmd.Run()

				if cfg.Verbose || v.Verbose {
					fmt.Println(outb.String())
					fmt.Println(errb.String())
				}
			}
		} else {
			if v.Log != "" {
				fmt.Printf(">> [Tar and Copy %v] %v:\n", idx, v.Log)
			} else {
				fmt.Printf(">> [Tar and Copy %v] Copying %v into %v:\n", idx, source, fmt.Sprintf("%v:%v", connectStr, dest))
			}

			if v.DryRun {
				filename = "/dev/null"
			} else {
				filename = "/tmp/" + filename
			}

			args := []string{excludes, "-zcvf", filename, source}

			fmt.Printf(">> [%v] RUN: %v\n", idx, strings.Join(append([]string{"tar"}, args...), " "))

			cmd := exec.Command("tar", args...)

			var outb, errb bytes.Buffer
			cmd.Stdout = &outb
			cmd.Stderr = &errb
			cmd.Run()

			if cfg.Verbose || v.Verbose {
				fmt.Println(outb.String())
				fmt.Println(errb.String())
			}

			// Downloading using SCP

			dest = fmt.Sprintf("%v:%v", connectStr, dest)
			args = []string{"-i", privateKey, "-P", port, filename, dest}

			fmt.Printf(">> [%v] RUN: %v\n", idx, strings.Join(append([]string{"scp"}, args...), " "))

			cmd = exec.Command("scp", args...)

			if !v.DryRun {
				var outb, errb bytes.Buffer
				cmd.Stdout = &outb
				cmd.Stderr = &errb
				cmd.Run()

				if cfg.Verbose || v.Verbose {
					fmt.Println(outb.String())
					fmt.Println(errb.String())
				}
			}
		}

		fmt.Printf(">> [%v] DONE! \n", idx)
	}
}

type progressWriter struct {
	total       *int
	progressBar *progressbar.ProgressBar
}

func (e progressWriter) Write(p []byte) (int, error) {
	matcher := regexp.MustCompile(`(?:^|\n)(\d+)###(.*)`)
	kk := matcher.FindAllSubmatch(p, -1)

	currentTotal := 0
	for i := 0; i < len(kk); i++ {
		if len(kk[i]) < 2 {
			continue
		}

		val, _ := strconv.Atoi(string(kk[i][1]))
		currentTotal += val
	}

	if e.progressBar != nil {
		e.progressBar.Add(currentTotal)

		if *e.total < 1 {
			e.progressBar.RenderBlank()
		}
	}
	*e.total += currentTotal

	return len(p), nil
}

func copyDirectories(cfg *Config) {
	if len(cfg.Sync) <= 0 {
		return
	}

	remoteMarker := regexp.MustCompile(`^remote:`)

	for index, syncItem := range cfg.Sync {
		fromRemote := remoteMarker.FindString(syncItem.Source) != ""
		source := remoteMarker.ReplaceAllString(syncItem.Source, "")
		dest := remoteMarker.ReplaceAllString(syncItem.Dest, "")

		server := cfg.Servers[syncItem.Server]
		connectStr := fmt.Sprintf("%v@%v", server.User, server.Host)
		idx := index + 1

		if fromRemote {
			source = fmt.Sprintf("%v:%v", connectStr, source)

			if syncItem.Log != "" {
				fmt.Printf(">> [%v] %v:\n", idx, syncItem.Log)
			} else {
				fmt.Printf(">> [%v] Copying %v into %v:\n", idx, source, dest)
			}

			excludes := parseExcludes(syncItem.Exclude)
			port := cfg.Servers[syncItem.Server].Port
			privateKey := cfg.Servers[syncItem.Server].PrivateKey
			sshArgs := "\"" + strings.Join([]string{"ssh", "-i", privateKey, "-p", port}, " ") + "\""

			args := []string{"-avz", "--progress", "--out-format=%l###%n"}

			if syncItem.DryRun {
				args = append(args, "--dry-run")
			}

			// if cfg.Verbose || v.Verbose {
			// 	args = append(args, "--stats")
			// }

			if syncItem.DeleteExtraneousFromDest {
				args = append(args, "--delete")
			}

			if excludes != "" {
				args = append(args, excludes)
			}

			args = append(args, source, dest)

			fmt.Printf(">> [%v] RUN: %v\n", idx, strings.Join(append([]string{"rsync"}, args...), " "))
			RsyncWithProgress(idx, args, sshArgs, cfg, syncItem)
		} else {
			dest = fmt.Sprintf("%v:%v", connectStr, dest)

			idx := index + 1
			if syncItem.Log != "" {
				fmt.Printf(">> [%v] %v:\n", idx, syncItem.Log)
			} else {
				fmt.Printf(">> [%v] Copying %v into %v:\n", idx, source, dest)
			}

			excludes := parseExcludes(syncItem.Exclude)
			port := cfg.Servers[syncItem.Server].Port
			privateKey := cfg.Servers[syncItem.Server].PrivateKey
			sshArgs := "\"" + strings.Join([]string{"ssh", "-i", privateKey, "-p", port}, " ") + "\""

			args := []string{"-avz", "--progress"}

			if syncItem.DryRun {
				args = append(args, "--dry-run")
			}

			if cfg.Verbose || syncItem.Verbose {
				args = append(args, "--stats")
			}

			if syncItem.DeleteExtraneousFromDest {
				args = append(args, "--delete")
			}

			args = append(args, excludes, source, dest)

			fmt.Printf(">> [%v] RUN: %v\n", idx, strings.Join(append([]string{"rsync"}, args...), " "))
			RsyncWithProgress(idx, args, sshArgs, cfg, syncItem)
		}

		fmt.Printf(">> [%v] DONE! \n", idx)
	}
}

func RsyncWithProgress(idx int, args []string, sshArgs string, cfg *Config, v struct {
	Server                   string "yaml:\"server\""
	Log                      string "yaml:\"log\""
	Source                   string "yaml:\"source\""
	Dest                     string "yaml:\"dest\""
	DeleteExtraneousFromDest bool   "yaml:\"delete_extraneous_from_dest\""
	Verbose                  bool   "yaml:\"verbose\""
	DryRun                   bool   "yaml:\"dry_run\""
	Exclude                  []string
}) {
	cmd := exec.Command("rsync", args...)

	// rsync-ssh crutch: no other way to use ssh for rsync here in golang :(
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "RSYNC_RSH=\""+sshArgs+"\"")
	// end rsync-ssh crutch

	total := executeRsyncPrediction(args, sshArgs)
	bar := createProgressbar(total, "syncing")

	dumbTotal := 0
	cmdWriter := &progressWriter{total: &dumbTotal, progressBar: bar}
	mw := io.MultiWriter(cmdWriter)
	if cfg.Verbose || v.Verbose {
		mw = io.MultiWriter(cmdWriter, os.Stdout)
	}

	cmd.Stdout = mw
	cmd.Stderr = mw

	cmd.Run()
	bar.Finish()
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

// ParseFlags will create and parse the CLI flags
// and return the path to be used elsewhere
func ParseFlags() (string, error) {
	// String that contains the configured configuration path
	var configPath string

	// Set up a CLI flag called "-config" to allow users
	// to supply the configuration file
	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")

	// Actually parse the flags
	flag.Parse()

	// Validate the path first
	if err := ValidateConfigPath(configPath); err != nil {
		return "", err
	}

	// Return the configuration path
	return configPath, nil
}

// NewConfig returns a new decoded Config struct
func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

// ValidateConfigPath just makes sure, that the path provided is a file,
// that can be read
func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%v' is a directory, not a normal file", path)
	}
	return nil
}
