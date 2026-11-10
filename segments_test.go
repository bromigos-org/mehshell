package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKubernetesContextFromListsAndQuotedValues(t *testing.T) {
	t.Setenv("TERM", "dumb")
	home, project, configs := t.TempDir(), t.TempDir(), t.TempDir()
	write := func(path, text string) string {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write(filepath.Join(home, ".kube", "config"), "apiVersion: v1\ncurrent-context: home-cluster\n")
	plain := write(filepath.Join(configs, "plain"), "apiVersion: v1\ncurrent-context: staging\n")
	double := write(filepath.Join(configs, "double"), "current-context: \"prod-east\"\n")
	single := write(filepath.Join(configs, "single"), "current-context: 'prod-west'\n")
	unset := write(filepath.Join(configs, "unset"), "apiVersion: v1\nclusters: []\n")
	empty := write(filepath.Join(configs, "empty"), "current-context: \"\"\n")
	missing := filepath.Join(configs, "missing")
	list := func(paths ...string) string { return strings.Join(paths, string(os.PathListSeparator)) }

	t.Setenv("KUBECONFIG", plain)
	if got := segK8s(project, home, false); got != "" {
		t.Fatalf("context shown without a manifest in the folder: %q", got)
	}
	write(filepath.Join(project, "Chart.yaml"), "name: example\n")
	for _, row := range []struct{ kubeconfig, want string }{
		{"", "k8s home-cluster"},
		{plain, "k8s staging"},
		{double, "k8s prod-east"},
		{single, "k8s prod-west"},
		{list(plain, double), "k8s staging"},
		{list(unset, double), "k8s prod-east"},
		{list(empty, missing, single), "k8s prod-west"},
		{list(unset, empty), ""},
		{missing, ""},
	} {
		t.Setenv("KUBECONFIG", row.kubeconfig)
		if got := segK8s(project, home, false); got != row.want {
			t.Errorf("KUBECONFIG=%q: got %q, want %q", row.kubeconfig, got, row.want)
		}
	}
}
