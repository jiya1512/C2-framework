package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	inputPath       string
	outputPath      string
	host            string
	port            string
	hostPlaceholder string
	portPlaceholder string
}

func main() {
	cfg := config{}
	flag.StringVar(&cfg.inputPath, "in", "", "Input file or directory containing payloads")
	flag.StringVar(&cfg.outputPath, "out", "", "Output file or directory (optional for in-place updates)")
	flag.StringVar(&cfg.host, "host", "", "Host value to insert into payloads (required)")
	flag.StringVar(&cfg.port, "port", "", "Port value to insert into payloads (required)")
	flag.StringVar(&cfg.hostPlaceholder, "host-placeholder", "{{LHOST}}", "Placeholder for host in payloads")
	flag.StringVar(&cfg.portPlaceholder, "port-placeholder", "{{LPORT}}", "Placeholder for port in payloads")
	flag.Parse()

	if cfg.inputPath == "" {
		exitWithError(errors.New("-in is required"))
	}
	if cfg.host == "" {
		exitWithError(errors.New("-host is required"))
	}
	if cfg.port == "" {
		exitWithError(errors.New("-port is required"))
	}

	info, err := os.Stat(cfg.inputPath)
	if err != nil {
		exitWithError(fmt.Errorf("stat input: %w", err))
	}

	if info.IsDir() {
		err = processDirectory(cfg)
	} else {
		err = processSingleFile(cfg, cfg.inputPath)
	}
	if err != nil {
		exitWithError(err)
	}
}

func processDirectory(cfg config) error {
	return filepath.WalkDir(cfg.inputPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		return processSingleFile(cfg, path)
	})
}

func processSingleFile(cfg config, inputPath string) error {
	content, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inputPath, err)
	}
	updated := strings.ReplaceAll(string(content), cfg.hostPlaceholder, cfg.host)
	updated = strings.ReplaceAll(updated, cfg.portPlaceholder, cfg.port)

	if updated == string(content) && cfg.outputPath == "" {
		return nil
	}

	outputPath, err := resolveOutputPath(cfg, inputPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputPath, err)
	}
	return nil
}

func resolveOutputPath(cfg config, inputPath string) (string, error) {
	if cfg.outputPath == "" {
		return inputPath, nil
	}

	outInfo, err := os.Stat(cfg.outputPath)
	if err == nil && outInfo.IsDir() {
		return filepath.Join(cfg.outputPath, filepath.Base(inputPath)), nil
	}

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat output: %w", err)
	}

	if !outInfoIsDir(outInfo, err) {
		if cfg.inputPath != inputPath {
			rel, relErr := filepath.Rel(cfg.inputPath, inputPath)
			if relErr == nil {
				return filepath.Join(cfg.outputPath, rel), nil
			}
		}
	}

	return cfg.outputPath, nil
}

func outInfoIsDir(info os.FileInfo, err error) bool {
	if err != nil || info == nil {
		return false
	}
	return info.IsDir()
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
