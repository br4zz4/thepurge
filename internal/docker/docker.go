// Package docker wraps the docker CLI for the optional cleanup phase.
package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"br4zz4/thepurge/internal/fsutil"
)

type Status int

const (
	StatusOK Status = iota
	StatusBinaryMissing
	StatusDaemonDown
)

type PruneKind int

const (
	PruneDangling PruneKind = iota
	PruneBuilder
)

func (k PruneKind) String() string {
	switch k {
	case PruneDangling:
		return "image prune"
	default:
		return "builder prune"
	}
}

type Reclaimable struct {
	Images     int64
	BuildCache int64
}

type PruneResult struct {
	FreedBytes int64
}

// Ops is the docker surface the engine depends on.
type Ops interface {
	Probe(ctx context.Context) (Status, error)
	Reclaimable(ctx context.Context) (Reclaimable, error)
	DanglingImageIDs(ctx context.Context) ([]string, error)
	Images(ctx context.Context) ([]Image, error)
	InUseImageIDs(ctx context.Context) (map[string]bool, error)
	RemoveImage(ctx context.Context, id string) error
	Prune(ctx context.Context, kind PruneKind) (PruneResult, error)
}

const (
	probeTimeout = 5 * time.Second
	pruneTimeout = 5 * time.Minute
	queryTimeout = 30 * time.Second
)

// Client runs the real docker binary. Bin defaults to "docker".
type Client struct {
	Bin string
}

func NewClient() *Client { return &Client{Bin: "docker"} }

// extraPaths are where docker commonly lives but launchd's minimal PATH
// (/usr/bin:/bin:/usr/sbin:/sbin) does not reach.
var ExtraPaths = []string{"/usr/local/bin", "/opt/homebrew/bin", "/opt/orbstack/bin", "/snap/bin"}

func (c *Client) bin() string {
	name := c.Bin
	if name == "" {
		name = "docker"
	}
	if strings.Contains(name, "/") {
		return name
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, dir := range ExtraPaths {
		p := dir + "/" + name
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return name
}

func (c *Client) run(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, c.bin(), args...).Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return string(out), err
}

func (c *Client) Probe(ctx context.Context) (Status, error) {
	if _, err := exec.LookPath(c.bin()); err != nil {
		return StatusBinaryMissing, nil
	}
	if _, err := c.run(ctx, probeTimeout, "version", "--format", "{{.Server.Version}}"); err != nil {
		return StatusDaemonDown, nil
	}
	return StatusOK, nil
}

func (c *Client) Reclaimable(ctx context.Context) (Reclaimable, error) {
	out, err := c.run(ctx, queryTimeout, "system", "df", "--format", "{{.Type}}\t{{.Reclaimable}}")
	if err != nil {
		return Reclaimable{}, err
	}
	var r Reclaimable
	for _, line := range strings.Split(out, "\n") {
		typ, rec, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(rec)
		if len(fields) == 0 {
			continue
		}
		n, err := fsutil.ParseBytes(fields[0])
		if err != nil {
			return Reclaimable{}, err
		}
		switch strings.TrimSpace(typ) {
		case "Images":
			r.Images = n
		case "Build Cache":
			r.BuildCache = n
		}
	}
	return r, nil
}

func (c *Client) DanglingImageIDs(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, queryTimeout, "image", "ls", "--filter", "dangling=true", "--format", "{{.ID}}")
	if err != nil {
		return nil, err
	}
	return strings.Fields(out), nil
}

func (c *Client) Prune(ctx context.Context, kind PruneKind) (PruneResult, error) {
	var args []string
	switch kind {
	case PruneDangling:
		args = []string{"image", "prune", "-f"}
	case PruneBuilder:
		args = []string{"builder", "prune", "-f"}
	default:
		return PruneResult{}, fmt.Errorf("unknown prune kind %d", kind)
	}
	out, err := c.run(ctx, pruneTimeout, args...)
	if err != nil {
		return PruneResult{}, err
	}
	return PruneResult{FreedBytes: parseReclaimed(out)}, nil
}

func parseReclaimed(out string) int64 {
	const marker = "Total reclaimed space:"
	for _, line := range strings.Split(out, "\n") {
		if _, rest, ok := strings.Cut(line, marker); ok {
			if n, err := fsutil.ParseBytes(strings.TrimSpace(rest)); err == nil {
				return n
			}
		}
	}
	return 0
}

// Image is a tagged local image.
type Image struct {
	ID   string
	Tags []string
	Size int64
}

// Label is the first tag, or the ID when untagged.
func (i Image) Label() string {
	if len(i.Tags) > 0 {
		return i.Tags[0]
	}
	return i.ID
}

// Images lists tagged images (dangling ones are left to Prune), one entry per ID.
func (c *Client) Images(ctx context.Context) ([]Image, error) {
	out, err := c.run(ctx, queryTimeout, "image", "ls", "--no-trunc", "--format", "{{.ID}}\t{{.Repository}}:{{.Tag}}\t{{.Size}}")
	if err != nil {
		return nil, err
	}
	byID := map[string]*Image{}
	var order []string
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(strings.TrimSpace(line), "\t")
		if len(parts) != 3 || strings.HasPrefix(parts[1], "<none>") {
			continue
		}
		img, ok := byID[parts[0]]
		if !ok {
			size, _ := fsutil.ParseBytes(parts[2])
			img = &Image{ID: parts[0], Size: size}
			byID[parts[0]] = img
			order = append(order, parts[0])
		}
		img.Tags = append(img.Tags, parts[1])
	}
	imgs := make([]Image, 0, len(order))
	for _, id := range order {
		imgs = append(imgs, *byID[id])
	}
	return imgs, nil
}

// InUseImageIDs returns the IDs of images referenced by any container,
// running or not.
func (c *Client) InUseImageIDs(ctx context.Context) (map[string]bool, error) {
	out, err := c.run(ctx, queryTimeout, "container", "ls", "-a", "-q", "--no-trunc")
	if err != nil {
		return nil, err
	}
	used := map[string]bool{}
	ids := strings.Fields(out)
	if len(ids) == 0 {
		return used, nil
	}
	out, err = c.run(ctx, queryTimeout, append([]string{"inspect", "--format", "{{.Image}}"}, ids...)...)
	if err != nil {
		return nil, err
	}
	for _, id := range strings.Fields(out) {
		used[id] = true
	}
	return used, nil
}

// RemoveImage removes one image without force, so docker refuses if it is in use.
func (c *Client) RemoveImage(ctx context.Context, id string) error {
	_, err := c.run(ctx, pruneTimeout, "image", "rm", id)
	return err
}
