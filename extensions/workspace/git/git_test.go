package gitworkspace

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	workspacecontract "github.com/thinkerqaq/devtool/sdk/workspace"
)

func TestWorkspaceLifecycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("worktree fixture expects unix-like paths")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	base := t.TempDir()
	root := filepath.Join(base, "repo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "init", "-q")
	runTestGit(t, root, "config", "user.name", "DevTool Test")
	runTestGit(t, root, "config", "user.email", "devtool@example.test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", "README.md")
	runTestGit(t, root, "commit", "-q", "-m", "initial")

	extension := New()
	ctx := context.Background()

	createRaw := invokeWorkspace(t, extension, ctx, workspacecontract.MethodCreate, workspacecontract.CreateRequest{
		Root:     root,
		Name:     "feature/one",
		Revision: "HEAD",
	})
	var created workspacecontract.CreateResponse
	if err := json.Unmarshal(createRaw, &created); err != nil {
		t.Fatal(err)
	}
	if created.Workspace.Provider != ExtensionID {
		t.Fatalf("provider = %q", created.Workspace.Provider)
	}
	if created.Workspace.Name != "feature/one" || created.Workspace.Primary {
		t.Fatalf("created workspace = %#v", created.Workspace)
	}
	if created.Workspace.Identity.RepositoryID == "" || created.Workspace.Identity.WorkspaceID == "" {
		t.Fatalf("missing workspace identity: %#v", created.Workspace.Identity)
	}
	if created.Workspace.Identity.Root == root {
		t.Fatal("linked worktree reused the primary root")
	}
	if _, err := os.Stat(created.Workspace.Identity.Root); err != nil {
		t.Fatalf("created workspace root: %v", err)
	}
	if branch := runTestGit(t, created.Workspace.Identity.Root, "branch", "--show-current"); branch != "feature/one" {
		t.Fatalf("branch = %q", branch)
	}

	listRaw := invokeWorkspace(t, extension, ctx, workspacecontract.MethodList, workspacecontract.ListRequest{Root: root})
	var listed workspacecontract.ListResponse
	if err := json.Unmarshal(listRaw, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Workspaces) != 2 {
		t.Fatalf("workspaces = %#v", listed.Workspaces)
	}
	if !listed.Workspaces[0].Primary {
		t.Fatalf("first worktree should be primary: %#v", listed.Workspaces[0])
	}
	if listed.Workspaces[0].Identity.RepositoryID != created.Workspace.Identity.RepositoryID {
		t.Fatal("linked worktree did not share repository identity")
	}
	if listed.Workspaces[0].Identity.WorkspaceID == created.Workspace.Identity.WorkspaceID {
		t.Fatal("primary and linked worktrees share workspace identity")
	}

	inspectRaw := invokeWorkspace(t, extension, ctx, workspacecontract.MethodInspect, workspacecontract.InspectRequest{
		Root:        root,
		WorkspaceID: created.Workspace.Identity.WorkspaceID,
	})
	var inspected workspacecontract.InspectResponse
	if err := json.Unmarshal(inspectRaw, &inspected); err != nil {
		t.Fatal(err)
	}
	if inspected.Workspace.Identity.Root != created.Workspace.Identity.Root {
		t.Fatalf("inspect = %#v", inspected.Workspace)
	}

	primaryRemove, _ := json.Marshal(workspacecontract.RemoveRequest{
		Root:        root,
		WorkspaceID: listed.Workspaces[0].Identity.WorkspaceID,
	})
	if _, err := extension.Invoke(ctx, workspacecontract.MethodRemove, primaryRemove); err == nil ||
		!strings.Contains(err.Error(), "primary workspace cannot be removed") {
		t.Fatalf("primary removal error = %v", err)
	}

	removeRaw := invokeWorkspace(t, extension, ctx, workspacecontract.MethodRemove, workspacecontract.RemoveRequest{
		Root:        root,
		WorkspaceID: created.Workspace.Identity.WorkspaceID,
	})
	var removed workspacecontract.RemoveResponse
	if err := json.Unmarshal(removeRaw, &removed); err != nil {
		t.Fatal(err)
	}
	if !removed.Removed || removed.WorkspaceID != created.Workspace.Identity.WorkspaceID {
		t.Fatalf("remove response = %#v", removed)
	}
	if _, err := os.Stat(created.Workspace.Identity.Root); !os.IsNotExist(err) {
		t.Fatalf("removed worktree still exists: %v", err)
	}

	listRaw = invokeWorkspace(t, extension, ctx, workspacecontract.MethodList, workspacecontract.ListRequest{Root: root})
	if err := json.Unmarshal(listRaw, &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Workspaces) != 1 || !listed.Workspaces[0].Primary {
		t.Fatalf("workspaces after remove = %#v", listed.Workspaces)
	}
}

func TestWorktreeDirectoryNameIsStableAndPathSafe(t *testing.T) {
	first := worktreeDirectoryName("feature/ui/layout")
	second := worktreeDirectoryName("feature/ui/layout")
	if first != second {
		t.Fatalf("directory name is not stable: %q != %q", first, second)
	}
	if strings.Contains(first, "/") || strings.Contains(first, "\\") {
		t.Fatalf("directory name is not path-safe: %q", first)
	}
}

func invokeWorkspace(
	t *testing.T,
	extension *Extension,
	ctx context.Context,
	method string,
	request any,
) json.RawMessage {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := extension.Invoke(ctx, method, payload)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func runTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}
