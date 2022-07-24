package main

import (
	"regexp"
	"strconv"

	"github.com/schollz/progressbar/v3"
)

type progressWriter struct {
	total       *int
	progressBar *progressbar.ProgressBar
}

func (e progressWriter) Write(p []byte) (int, error) {
	matcher := regexp.MustCompile(`(?:^|\n)(\d+)###(.*)`)
	kk := matcher.FindAllSubmatch(p, -1)
	total := e.total
	currentTotal := 0
	if total == nil {
		totalContainer := 0
		total = &totalContainer
	}

	for i := 0; i < len(kk); i++ {
		if len(kk[i]) < 2 {
			continue
		}

		val, _ := strconv.Atoi(string(kk[i][1]))
		currentTotal += val
	}

	if e.progressBar != nil {
		e.progressBar.Add(currentTotal)

		if *total < 1 {
			e.progressBar.RenderBlank()
		}
	}
	*total += currentTotal

	return len(p), nil
}
