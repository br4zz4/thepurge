package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"br4zz4/thepurge/config"
)

// defaultLogLines is how many tail lines `thepurge logs` shows by default.
const defaultLogLines = 50

// logPathFor returns the log file the CLI writes/reads. It honours
// config logging.log_path and otherwise falls back to the platform default.
func logPathFor(cfg *config.Config) string {
	if cfg != nil && cfg.Logging.LogPath != "" {
		return expandHome(cfg.Logging.LogPath)
	}
	return expandHome(config.Default().Logging.LogPath)
}

func expandHome(p string) string {
	if p == "~" {
		h, err := os.UserHomeDir()
		if err == nil {
			return h
		}
	}
	if strings.HasPrefix(p, "~/") {
		h, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// parseLogFlags extracts -n/--n/--n=N and -t/--tail from args.
func parseLogFlags(args []string) (n int, tail bool) {
	n = defaultLogLines
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-n" || a == "--n":
			if i+1 < len(args) {
				if v, err := strconv.Atoi(args[i+1]); err == nil && v > 0 {
					n = v
				}
				i++
			}
		case strings.HasPrefix(a, "--n=") || strings.HasPrefix(a, "-n="):
			if v, err := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(a, "--n="), "-n=")); err == nil && v > 0 {
				n = v
			}
		case a == "-t" || a == "--tail":
			tail = true
		}
	}
	return n, tail
}

// readTail returns the last n lines of path. A missing file yields no error.
func readTail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	// Keep a sliding window of the last n lines.
	window := make([]string, 0, n)
	for sc.Scan() {
		window = append(window, sc.Text())
		if len(window) > n {
			window = window[1:]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return window, nil
}

// followTail streams new lines appended to path until the process is stopped.
func followTail(path string, out io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(out, "no log file yet at %s\n", path)
			return nil
		}
		return err
	}
	defer f.Close()

	// Seek to the end to see only new entries.
	last := int64(0)
	if st, err := f.Stat(); err == nil {
		last = st.Size()
	}

	rd := bufio.NewReader(f)
	for {
		if _, err := f.Seek(last, io.SeekStart); err != nil {
			return err
		}
		for {
			line, err := rd.ReadString('\n')
			if line != "" {
				fmt.Fprint(out, line)
			}
			if err != nil {
				break
			}
		}
		if st, err := f.Stat(); err == nil {
			last = st.Size()
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// cmdLogs prints recent log entries (-n N) or follows the log (-t/--tail).
func cmdLogs(args []string, out, errOut io.Writer) (int, error) {
	n, tail := parseLogFlags(args)
	path := logPathFor(configOrNil())

	if tail {
		return 0, followTail(path, out)
	}

	lines, err := readTail(path, n)
	if err != nil {
		return 1, err
	}
	if len(lines) == 0 {
		fmt.Fprintf(out, "no log entries yet at %s\n", path)
		return 0, nil
	}
	for _, l := range lines {
		fmt.Fprintln(out, l)
	}
	return 0, nil
}

// configOrNil loads the config for log path resolution, returning nil on any
// failure so `logs` still works without a valid config.
func configOrNil() *config.Config {
	cfg, _, err := loadConfig()
	if err != nil {
		return nil
	}
	return cfg
}
