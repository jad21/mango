//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestNewProcessBuildsAndRuns validates that a Process starts in the given workdir,
// receives environment variables, and responds to SIGTERM via Process.Signal.
func TestNewProcessBuildsAndRuns(t *testing.T) {
	// Crear un directorio temporal como workdir
	workdir := t.TempDir()

	// Archivo que el subproceso creará dentro de workdir
	outFile := "foo.txt"

	// Comando de shell:
	//  - Escribe $FOO en foo.txt
	//  - Atrapa SIGTERM y anota "TERM" en foo.txt
	//  - Permanece en bucle hasta recibir la señal
	cmd := strings.Join([]string{
		`printf "%s\n" "$FOO" > ` + outFile + `;`,
		`trap 'printf "TERM\n" >> ` + outFile + `; exit 0' TERM;`,
		`while :; do sleep 1; done`,
	}, " ")

	// Env a inyectar
	env := make(Env)
	env["FOO"] = "bar"

	// Crear proceso (no interactivo)
	p := NewProcess(workdir, cmd, env, false)
	if p == nil || p.Cmd == nil {
		t.Fatalf("NewProcess returned nil")
	}

	// Aserción ligera sobre argv (debería invocar sh -c ...)
	if len(p.Args) < 2 || p.Args[0] != "sh" {
		t.Fatalf("expected shell invocation, got args: %#v", p.Args)
	}

	// Arrancar
	if err := p.Start(); err != nil {
		t.Fatalf("failed to start process: %v", err)
	}

	// Dar un tiempo para que escriba el valor de $FOO
	time.Sleep(200 * time.Millisecond)

	// Verificar que se creó el archivo con el valor del env
	data, err := os.ReadFile(filepath.Join(workdir, outFile))
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "bar" {
		t.Fatalf("unexpected file content before TERM: %q (want %q)", got, "bar")
	}

	// Enviar SIGTERM al grupo de proceso mediante Process.Signal
	if err := p.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("failed to send SIGTERM: %v", err)
	}

	// Esperar salida con timeout prudente
	done := make(chan error, 1)
	go func() {
		done <- p.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			// En Linux, terminar por SIGTERM suele reflejarse como error "signal: terminated"
			// para Cmd.Wait(). Lo importante aquí es que terminó y ejecutó el trap.
		}
	case <-time.After(3 * time.Second):
		t.Fatal("process did not exit after SIGTERM")
	}

	// Verificar que el trap corrió y se añadió "TERM"
	data2, err := os.ReadFile(filepath.Join(workdir, outFile))
	if err != nil {
		t.Fatalf("failed to read output file after TERM: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data2)), "\n")
	if len(lines) < 2 || lines[len(lines)-1] != "TERM" {
		t.Fatalf("expected last line to be TERM, got: %q", string(data2))
	}
}
