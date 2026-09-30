package docker_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/prunesh/docker/filter"
)

// --- Rewrite ---

func TestRewriteLogsAddsTail(t *testing.T) {
	got, ok := filter.Rewrite([]string{"logs", "my-container"})
	if !ok {
		t.Fatal("expected rewrite for logs")
	}
	var found bool
	for _, a := range got {
		if strings.HasPrefix(a, "--tail=") {
			found = true
		}
	}
	if !found {
		t.Fatalf("--tail flag not injected: %v", got)
	}
}

func TestRewriteLogsNoRewriteWithFollow(t *testing.T) {
	_, ok := filter.Rewrite([]string{"logs", "-f", "my-container"})
	if ok {
		t.Fatal("must not rewrite logs with -f")
	}
}

func TestRewriteLogsNoRewriteWithTail(t *testing.T) {
	_, ok := filter.Rewrite([]string{"logs", "--tail=200", "my-container"})
	if ok {
		t.Fatal("must not rewrite logs when --tail is already set")
	}
}

func TestRewriteRunNoRewrite(t *testing.T) {
	_, ok := filter.Rewrite([]string{"run", "-it", "ubuntu"})
	if ok {
		t.Fatal("must not rewrite run")
	}
}

func TestRewritePullNoRewrite(t *testing.T) {
	_, ok := filter.Rewrite([]string{"pull", "nginx:latest"})
	if ok {
		t.Fatal("must not rewrite pull")
	}
}

// --- FilterOutput: inspect ---

const inspectOutput = `[
    {
        "Id": "abc123def456",
        "Created": "2026-08-29T10:00:00.000000000Z",
        "Path": "/bin/sh",
        "Args": [],
        "State": {
            "Status": "running",
            "Running": true,
            "Pid": 1234,
            "ExitCode": 0
        },
        "Image": "sha256:abc123",
        "Name": "/my-container",
        "Config": {
            "Hostname": "abc123",
            "Image": "nginx:latest",
            "Labels": {
                "app": "web"
            }
        },
        "HostConfig": {
            "Binds": null,
            "NetworkMode": "default",
            "PortBindings": {}
        },
        "GraphDriver": {
            "Data": {
                "LowerDir": "/var/lib/docker/overlay2/abc/diff",
                "MergedDir": "/var/lib/docker/overlay2/abc/merged"
            },
            "Name": "overlay2"
        },
        "Mounts": [],
        "NetworkSettings": {
            "Bridge": "",
            "IPAddress": "172.17.0.2",
            "Ports": {}
        },
        "LogPath": "/var/lib/docker/containers/abc123/abc123-json.log",
        "ResolvConfPath": "/etc/resolv.conf"
    }
]
`

func TestFilterInspectStripsGraphDriver(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	if strings.Contains(out, "GraphDriver") {
		t.Error("filtered inspect must not contain GraphDriver")
	}
}

func TestFilterInspectStripsHostConfig(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	if strings.Contains(out, "HostConfig") {
		t.Error("filtered inspect must not contain HostConfig")
	}
}

func TestFilterInspectStripsNetworkSettings(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	if strings.Contains(out, "NetworkSettings") {
		t.Error("filtered inspect must not contain NetworkSettings")
	}
}

func TestFilterInspectStripsLogPath(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	if strings.Contains(out, "LogPath") {
		t.Error("filtered inspect must not contain LogPath")
	}
}

func TestFilterInspectKeepsEssential(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	for _, want := range []string{"State", "Name", "Config", "Image"} {
		if !strings.Contains(out, want) {
			t.Errorf("filtered inspect must contain %q", want)
		}
	}
}

func TestFilterInspectIsSmaller(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	if len(out) >= len(inspectOutput) {
		t.Errorf("filtered inspect (%d bytes) must be smaller than original (%d bytes)", len(out), len(inspectOutput))
	}
}

func TestFilterInspectValidJSON(t *testing.T) {
	out := filter.FilterOutput([]string{"inspect", "my-container"}, inspectOutput, 0)
	var v interface{}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Errorf("filtered inspect is not valid JSON: %v\noutput:\n%s", err, out)
	}
}

// --- FilterOutput: pull ---

const pullOutput = `latest: Pulling from library/nginx
Pulling fs layer
Waiting
Downloading   1.23MB/50.00MB
Verifying Checksum
Download complete
Pull complete
Pulling fs layer
Pull complete
Digest: sha256:abc123def456
Status: Downloaded newer image for nginx:latest
docker.io/library/nginx:latest
`

func TestFilterPullStripsLayerNoise(t *testing.T) {
	out := filter.FilterOutput([]string{"pull", "nginx:latest"}, pullOutput, 0)
	for _, noise := range []string{"Pulling fs layer", "Waiting", "Verifying Checksum", "Download complete", "Pull complete"} {
		if strings.Contains(out, noise) {
			t.Errorf("filtered pull must not contain %q", noise)
		}
	}
}

func TestFilterPullKeepsStatus(t *testing.T) {
	out := filter.FilterOutput([]string{"pull", "nginx:latest"}, pullOutput, 0)
	if !strings.Contains(out, "Status:") {
		t.Error("filtered pull must keep Status line")
	}
}

func TestFilterPullIsSmaller(t *testing.T) {
	out := filter.FilterOutput([]string{"pull", "nginx:latest"}, pullOutput, 0)
	if len(out) >= len(pullOutput) {
		t.Errorf("filtered pull (%d bytes) must be smaller than original (%d bytes)", len(out), len(pullOutput))
	}
}

// --- FilterOutput: images ---

func makeImagesOutput(n int) string {
	lines := []string{"REPOSITORY          TAG       IMAGE ID       CREATED        SIZE"}
	for i := 0; i < n; i++ {
		lines = append(lines, "nginx               latest    abc123def456   2 days ago     187MB")
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestFilterImagesPassthroughShortList(t *testing.T) {
	input := makeImagesOutput(5)
	out := filter.FilterOutput([]string{"images"}, input, 0)
	if out != input {
		t.Error("short images list must pass through unchanged")
	}
}

func TestFilterImagesCapLongList(t *testing.T) {
	input := makeImagesOutput(50)
	out := filter.FilterOutput([]string{"images"}, input, 0)
	if len(out) >= len(input) {
		t.Errorf("long images list must be capped")
	}
	if !strings.Contains(out, "more images") {
		t.Error("capped images list must include '... +N more images' line")
	}
}

// --- FilterOutput: passthrough ---

func TestFilterRunPassthrough(t *testing.T) {
	input := "abc123def456\n"
	out := filter.FilterOutput([]string{"run", "-d", "nginx"}, input, 0)
	if out != input {
		t.Error("run must pass through unchanged")
	}
}

func TestFilterPsPassthrough(t *testing.T) {
	input := "CONTAINER ID   IMAGE     COMMAND   CREATED   STATUS    PORTS     NAMES\n"
	out := filter.FilterOutput([]string{"ps"}, input, 0)
	if out != input {
		t.Error("ps must pass through unchanged")
	}
}

// --- ID constant ---

func TestID(t *testing.T) {
	if filter.ID != "prunesh/docker" {
		t.Fatalf("ID %q does not follow author/<cmd> rule", filter.ID)
	}
}

// --- prunesh.toml manifest ---

func TestManifest(t *testing.T) {
	data, err := os.ReadFile("prunesh.toml")
	if err != nil {
		t.Fatalf("read prunesh.toml: %v", err)
	}
	manifest := string(data)
	for _, want := range []string{
		`id = "prunesh/docker"`,
		`command = "docker"`,
		`platforms = [`,
		`contract = "stdin/v1"`,
		`[prunesh-core-version]`,
		`version = "0.16.0"`,
		`constraint = "min"`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest must contain %q", want)
		}
	}
}
