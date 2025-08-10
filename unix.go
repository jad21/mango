//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

package main

import (
	"fmt"
	"syscall"
)

const osHaveSigTerm = true

// func ShellInvocationCommand(interactive bool, root, command string) []string {
// 	shellArgument := "-c"
// 	if interactive {
// 		shellArgument = "-ic"
// 	}
// 	shellCommand := fmt.Sprintf("cd \"%s\"; source .profile 2>/dev/null; exec %s", root, command)
// 	return []string{"sh", shellArgument, shellCommand}
// }

// func ShellInvocationCommand(interactive bool, root, command string) []string {
// 	shellArg := "-c"
// 	if interactive {
// 		shellArg = "-ic"
// 	}
// 	// POSIX: usar "." en lugar de "source".
// 	// Además, ejecuta SIEMPRE el comando bajo "sh -c <command>",
// 	// así el Procfile no necesita envolverlo en 'sh -c'.
// 	shellCommand := fmt.Sprintf(
// 		`cd %q; . .profile 2>/dev/null || true; exec sh -c %q`,
// 		root, command,
// 	)
// 	return []string{"sh", shellArg, shellCommand}
// }

func ShellInvocationCommand(interactive bool, root, command string) []string {
	// Un único sh -c, sin exec delante del comando del Procfile
	// para permitir construcciones de shell (while, if, pipes, etc.).
	shellArg := "-c"
	shellCommand := fmt.Sprintf(`cd %q; %s`, root, command)
	return []string{"sh", shellArg, shellCommand}
}

func (p *Process) PlatformSpecificInit() {
	if !p.Interactive {
		p.SysProcAttr = &syscall.SysProcAttr{}
		p.SysProcAttr.Setsid = true
	}
}

func (p *Process) SendSigTerm() {
	p.Signal(syscall.SIGTERM)
}

func (p *Process) SendSigKill() {
	p.Signal(syscall.SIGKILL)
}
