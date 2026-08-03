package repository

import "testing"

func TestNewExecuteOptionsUsesDefaultOutputDirectory(t *testing.T) {
	options := NewExecuteOptions()

	if options.OutputDirectory != outputDirName {
		t.Fatalf("OutputDirectory = %q, want %q", options.OutputDirectory, outputDirName)
	}
}

func TestWithOutputDirectory(t *testing.T) {
	const outputDirectory = "/tmp/netmigo/ssh_command_outputs"

	options := NewExecuteOptions(WithOutputDirectory(outputDirectory))

	if options.OutputDirectory != outputDirectory {
		t.Fatalf("OutputDirectory = %q, want %q", options.OutputDirectory, outputDirectory)
	}
}

func TestResolveOutputDirectoryFallsBackForEmptyPath(t *testing.T) {
	if got := resolveOutputDirectory(""); got != outputDirName {
		t.Fatalf("resolveOutputDirectory(\"\") = %q, want %q", got, outputDirName)
	}
}
