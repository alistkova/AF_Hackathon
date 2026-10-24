package content

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
)

const maxPDFPages = 500

var ErrPDFUnavailable = errors.New("pdf text extraction is not available")

func ExtractPDFText(ctx context.Context, data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("empty pdf")
	}

	binary := pdfTextBinary()
	if _, err := exec.LookPath(binary); err != nil {
		return "", ErrPDFUnavailable
	}

	command := exec.CommandContext(ctx, binary,
		"-enc", "UTF-8",
		"-nopgbrk",
		"-f", "1",
		"-l", fmt.Sprint(maxPDFPages),
		"-", "-",
	)
	command.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("pdftotext: %s", message)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func pdfTextBinary() string {
	return config.String("PDFTOTEXT_BIN", "pdftotext")
}
