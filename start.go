package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const defaultPort = 0
const defaultShutdownGraceTime = 3

var flagPort int
var flagConcurrency string
var flagRestart bool
var flagShutdownGraceTime int
var envs envFiles

// Nuevos flags para Loki
var flagLokiURL string
var flagLokiJob string

var lokiClient *LokiClient

var cmdStart = &Command{
	Run:   runStart,
	Usage: "start [process name] [-f procfile] [-e env] [-p port] [-c concurrency] [-r] [-t shutdown_grace_time]",
	Short: "Start the application",
	Long: `
Start the application specified by a Procfile. The directory containing the
Procfile is used as the working directory.

The following options are available:

  -f procfile  Set the Procfile. Defaults to './Procfile'.

  -e env       Add an environment file, containing variables in 'KEY=value', or
               'export KEY=value', form. These variables will be set in the
               environment of each process. If no environment files are
               specified, a file called .env is used if it exists.

  -p port      Sets the base port number; each process will have a PORT variable
               in its environment set to a unique value based on this. This may
               also be set via a PORT variable in the environment, or in an
               environment file, and otherwise defaults to 5000.

  -c concurrency
               Start a specific number of instances of each process. The
               argument should be in the format 'foo=1,bar=2,baz=0'. Use the
               name 'all' to set the default number of instances. By default,
               one instance of each process is started.

  -r           Restart a process which exits. Without this, if a process exits,
               mango will kill all other processes and exit.

  -t shutdown_grace_time
               Set the shutdown grace time that each process is given after
               being asked to stop. Once this grace time expires, the process is
               forcibly terminated. By default, it is 3 seconds.

If there is a file named .mango in the current directory, it will be read in
the same way as an environment file, and the values of variables procfile, port,
concurrency, and shutdown_grace_time used to change the corresponding default
values.

Examples:

  # start every process
  mango start

  # start only the web process
  mango start web

  # start every process specified in Procfile.test, with the environment specified in .env.test
  mango start -f Procfile.test -e .env.test

  # start every process, with a timeout of 30 seconds
  mango start -t 30
`,
}

func init() {
	cmdStart.Flag.StringVar(&flagProcfile, "f", "Procfile", "procfile")
	cmdStart.Flag.Var(&envs, "e", "env")
	cmdStart.Flag.IntVar(&flagPort, "p", defaultPort, "port")
	cmdStart.Flag.StringVar(&flagConcurrency, "c", "", "concurrency")
	cmdStart.Flag.BoolVar(&flagRestart, "r", false, "restart")
	cmdStart.Flag.IntVar(&flagShutdownGraceTime, "t", defaultShutdownGraceTime, "shutdown grace time")

	// Registrar flags de Loki
	cmdStart.Flag.StringVar(&flagLokiURL, "loki.url", "", "URL de Loki (ej: http://localhost:3100)")
	cmdStart.Flag.StringVar(&flagLokiJob, "loki.job", "forego", "Etiqueta job para Loki")

	err := readConfigFile(
		".mango",
		&flagProcfile,
		&flagPort,
		&flagConcurrency,
		&flagShutdownGraceTime,
		&flagLokiURL,
		&flagLokiJob,
	)
	handleError(err)
}

func readConfigFile(
	config_path string,
	flagProcfile *string,
	flagPort *int,
	flagConcurrency *string,
	flagShutdownGraceTime *int,
	flagLokiURL *string,
	flagLokiJob *string,
) error {
	config, err := ReadConfig(config_path)

	if config["procfile"] != "" {
		*flagProcfile = config["procfile"]
	} else {
		*flagProcfile = "Procfile"
	}
	if config["port"] != "" {
		*flagPort, err = strconv.Atoi(config["port"])
	} else {
		*flagPort = defaultPort
	}
	if config["shutdown_grace_time"] != "" {
		*flagShutdownGraceTime, err = strconv.Atoi(config["shutdown_grace_time"])
	} else {
		*flagShutdownGraceTime = defaultShutdownGraceTime
	}
	*flagConcurrency = config["concurrency"]

	if config["loki.url"] != "" {
		*flagLokiURL = config["loki.url"]
	}
	if config["loki.job"] != "" {
		*flagLokiJob = config["loki.job"]
	}
	return err
}

func parseConcurrency(value string) (map[string]int, error) {
	concurrency := map[string]int{}
	if strings.TrimSpace(value) == "" {
		return concurrency, nil
	}

	parts := strings.Split(value, ",")
	for _, part := range parts {
		if !strings.Contains(part, "=") {
			return concurrency, errors.New("concurrency should be in the format: foo=1,bar=2")
		}

		nameValue := strings.Split(part, "=")
		n, v := strings.TrimSpace(nameValue[0]), strings.TrimSpace(nameValue[1])
		if n == "" || v == "" {
			return concurrency, errors.New("concurrency should be in the format: foo=1,bar=2")
		}

		numProcs, err := strconv.ParseInt(v, 10, 16)
		if err != nil {
			return concurrency, err
		}

		concurrency[n] = int(numProcs)
	}
	return concurrency, nil
}

type mango struct {
	outletFactory *OutletFactory

	teardown, teardownNow Barrier // signal shutting down

	wg sync.WaitGroup
}

func (f *mango) monitorInterrupt() {
	handler := make(chan os.Signal, 1)
	signal.Notify(handler, syscall.SIGALRM, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	first := true

	for sig := range handler {
		fmt.Printf("mango    | monitorInterrupt: got signal %s\n", sig) // ← log
		switch sig {
		case syscall.SIGINT:
			fmt.Println("      | ctrl-c detected")
			fallthrough
		default:
			f.teardown.Fall()
			if !first {
				f.teardownNow.Fall()
			}
			first = false
		}
	}
}

func basePort(env Env) (int, error) {
	if flagPort != defaultPort {
		return flagPort, nil
	} else if env["PORT"] != "" {
		return strconv.Atoi(env["PORT"])
	} else if os.Getenv("PORT") != "" {
		return strconv.Atoi(os.Getenv("PORT"))
	}
	return defaultPort, nil
}

func (f *mango) startProcess(idx, procNum int, proc ProcfileEntry, env Env, of *OutletFactory) {
	// ===== entorno por proceso =====
	envCopy := env.Clone()

	// Puerto base
	port, err := basePort(envCopy)
	if err != nil {
		panic(err)
	}
	if port > 0 {
		port += idx * 100
		envCopy["PORT"] = strconv.Itoa(port)
	}

	// Proceso
	const interactive = false
	workDir := filepath.Dir(flagProcfile)
	ps := NewProcess(workDir, proc.Command, envCopy, interactive)

	// Nombre visible
	var procName string
	if procNum > 1 {
		procName = fmt.Sprintf("%s.%d", proc.Name, procNum+1)
	} else {
		procName = proc.Name
	}

	// Pipes
	stdout, err := ps.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := ps.StderrPipe()
	if err != nil {
		panic(err)
	}

	pipeWait := new(sync.WaitGroup)

	// --- Stdout ---
	pipeWait.Add(1)
	go func() {
		var reader io.Reader = stdout
		if lokiClient != nil {
			pr, pw := io.Pipe()
			reader = io.TeeReader(stdout, pw)
			go func() {
				scanner := bufio.NewScanner(pr)
				for scanner.Scan() {
					lokiClient.Send(flagLokiJob, procName, scanner.Text())
				}
				_ = pr.Close()
			}()
			defer pw.Close()
		}
		of.LineReader(pipeWait, procName, idx, reader, false)
	}()

	// --- Stderr ---
	pipeWait.Add(1)
	go func() {
		var reader io.Reader = stderr
		if lokiClient != nil {
			pr, pw := io.Pipe()
			reader = io.TeeReader(stderr, pw)
			go func() {
				scanner := bufio.NewScanner(pr)
				for scanner.Scan() {
					lokiClient.Send(flagLokiJob, procName, scanner.Text())
				}
				_ = pr.Close()
			}()
			defer pw.Close()
		}
		of.LineReader(pipeWait, procName, idx, reader, true)
	}()

	if port > 0 {
		of.SystemOutput(fmt.Sprintf("starting %s on port %d", procName, port))
	} else {
		of.SystemOutput(fmt.Sprintf("starting %s", procName))
	}

	// Señal de finalización de I/O + proceso
	finished := make(chan struct{})

	// ===== Start =====
	err = ps.Start()
	if err != nil {
		of.SystemOutput(fmt.Sprintf("Failed to start %s: %v", procName, err))
		of.SystemOutput(fmt.Sprintf("teardown cause: start-error (%s)", procName))
		f.teardown.Fall() // ← log explícito del origen
		return
	}

	// ===== Espera de I/O + Wait() con logging detallado =====
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer close(finished)

		// Espera a que terminen lectores
		pipeWait.Wait()

		// Espera del proceso
		waitErr := ps.Wait()

		// Log de salida: código o señal
		if waitErr != nil {
			of.SystemOutput(fmt.Sprintf("%s exited with error: %v", procName, waitErr))
		}

		if ps.ProcessState != nil {
			if status, ok := ps.ProcessState.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					of.SystemOutput(fmt.Sprintf("%s exit signal: %s", procName, status.Signal()))
				} else {
					of.SystemOutput(fmt.Sprintf("%s exit code: %d", procName, status.ExitStatus()))
				}
			} else {
				of.SystemOutput(fmt.Sprintf("%s exited (state: %v)", procName, ps.ProcessState))
			}
		} else {
			of.SystemOutput(fmt.Sprintf("%s exited (no process state)", procName))
		}
	}()

	// ===== Política de restart/teardown =====
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()

		select {
		case <-finished:
			if flagRestart {
				of.SystemOutput(fmt.Sprintf("restart policy: restarting %s", procName))
				// Reinicio del mismo proceso (mismo idx/procNum)
				f.startProcess(idx, procNum, proc, env, of)
			} else {
				of.SystemOutput(fmt.Sprintf("teardown cause: %s finished (no -r)", procName))
				f.teardown.Fall()
			}

		case <-f.teardown.Barrier():
			// Teardown global
			of.SystemOutput(fmt.Sprintf("teardown path: sending SIGTERM to %s", procName))
			if !osHaveSigTerm {
				of.SystemOutput(fmt.Sprintf("Killing %s", procName))
				_ = ps.Process.Kill()
				return
			}

			ps.SendSigTerm()

			// Grace period o salida del proceso
			select {
			case <-f.teardownNow.Barrier():
				of.SystemOutput(fmt.Sprintf("Killing %s", procName))
				ps.SendSigKill()
			case <-finished:
			}
		}
	}()
}

func runStart(cmd *Command, args []string) {
	pf, err := ReadProcfile(flagProcfile)
	handleError(err)

	concurrency, err := parseConcurrency(flagConcurrency)
	handleError(err)

	env, err := loadEnvs(envs)
	handleError(err)

	of := NewOutletFactory()
	of.Padding = pf.LongestProcessName(concurrency)

	f := &mango{
		outletFactory: of,
	}

	// ==== Inicializar cliente de Loki sólo si se ha configurado URL ====
	if flagLokiURL != "" {
		// lokiClient = NewLokiClient(flagLokiURL, 10*time.Second)
		// of.SystemOutput(fmt.Sprintf("Loki habilitado: %s (job=%s)", flagLokiURL, flagLokiJob))
		initLoki(of)
		handleError(err)
		defer lokiClient.Close()
	}

	go f.monitorInterrupt()

	// When teardown fires, start the grace timer
	f.teardown.FallHook = func() {
		go func() {
			time.Sleep(time.Duration(flagShutdownGraceTime) * time.Second)
			of.SystemOutput("Grace time expired")
			f.teardownNow.Fall()
		}()
	}

	var singleton string = ""
	if len(args) > 0 {
		singleton = args[0]
		if !pf.HasProcess(singleton) {
			of.ErrorOutput(fmt.Sprintf("no such process: %s", singleton))
		}
	}

	defaultConcurrency := 1

	for name, num := range concurrency {
		if name == "all" {
			defaultConcurrency = num
		}
	}

	for idx, proc := range pf.Entries {
		numProcs := defaultConcurrency
		if len(concurrency) > 0 {
			if value, ok := concurrency[proc.Name]; ok {
				numProcs = value
			}
		}
		for i := 0; i < numProcs; i++ {
			if (singleton == "") || (singleton == proc.Name) {
				f.startProcess(idx, i, proc, env, of)
			}
		}
	}

	<-f.teardown.Barrier()

	f.wg.Wait()
}

// initLoki inicializa el cliente de Loki y espera a que esté listo antes de continuar.
func initLoki(of *OutletFactory) {
	if flagLokiURL == "" || lokiClient != nil {
		return
	}
	lokiClient = NewLokiClient(flagLokiURL, 10*time.Second, 1*time.Second, 500)
	of.SystemOutput(fmt.Sprintf("Loki habilitado: %s (job=%s)", flagLokiURL, flagLokiJob))

	// Espera readiness
	of.SystemOutput("Esperando a que Loki esté listo...")
	if err := lokiClient.WaitReady(10, 1*time.Second); err != nil {
		of.SystemOutput("Aviso: no se pudo verificar readiness de Loki: " + err.Error())
	} else {
		of.SystemOutput("Loki está listo")
	}
}
