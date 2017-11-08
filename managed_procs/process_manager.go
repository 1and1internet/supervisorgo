package managed_procs

import (
	"os/exec"
	"log"
	"os"
	"time"
	"strings"
	"reflect"
	"fmt"
	"syscall"
)

type ProcStatus int

// Ref: http://supervisord.org/subprocess.html#process-states
const (
	PROC_STOPPED  ProcStatus = iota
	PROC_STARTING
	PROC_RUNNING
	PROC_BACKOFF
	PROC_EXITED
	PROC_FATAL
	PROC_STOPPING
)

type Program struct {
	config                 ProgramConfigSection
	programStatus          ProcStatus
	exitStatus             string
	startCount             int
	stdout                 *os.File
	stderr                 *os.File
	channel                chan ProcStatus
	commandPath            string
	programStatusTimestamp time.Time
	command                *exec.Cmd
	startable              bool
}

type RunningData struct {
	programs  []*Program
	allConfig AllConfig
}

func stateToString(state ProcStatus) (string) {
	switch state {
	case PROC_STOPPED:
		return "STOPPED"
	case PROC_RUNNING:
		return "RUNNING"
	case PROC_STARTING:
		return "STARTING"
	case PROC_FATAL:
		return "FATAL"
	case PROC_BACKOFF:
		return "BACKOFF"
	case PROC_EXITED:
		return "EXITED"
	case PROC_STOPPING:
		return "STOPPING"
	}
	return "unknown"
}

func (program *Program) UpdateStatus(status ProcStatus) {
	program.programStatus = status
	program.programStatusTimestamp = time.Now()
	log.Printf("Process '%s' changed state to '%s'\n",
		program.config.ProcessName,
		stateToString(program.programStatus))
}

func (allConfig AllConfig) InitialiseProcesses() []*Program {
	programs := []*Program{}
	for _, programConfig := range allConfig.Programs {
		aProgram := Program{
			config:     programConfig,
			exitStatus: "",
			startCount: 0,
			channel:    make(chan ProcStatus),
			startable:  false,
		}
		aProgram.UpdateStatus(PROC_STOPPED)

		if len(aProgram.config.Command) == 0 {
			log.Printf("No command specified for %s\n", aProgram.config.ProcessName)
			aProgram.UpdateStatus(PROC_FATAL)
			continue
		}

		path, err := exec.LookPath(aProgram.config.Command[0])
		if err != nil {
			log.Printf("Could not find command: %s\n", err)
			aProgram.UpdateStatus(PROC_FATAL)
			continue
		}
		aProgram.commandPath = path
		aProgram.channel = make(chan ProcStatus)
		programs = append(programs, &aProgram)
	}
	return programs
}

func (allConfig AllConfig) RunAllProcesses() {
	runningData := RunningData{
		programs: allConfig.InitialiseProcesses(),
		allConfig: allConfig,
	}
	for _, prog := range runningData.programs {
		if prog.config.AutoStart {
			prog.startable = true
			prog.StartRunableProcess()
		}
	}
	runningData.SignalHandlers()
	runningData.MonitorRunningProcesses()
}

func (runningData RunningData) MonitorRunningProcesses() {
	var chans []chan ProcStatus
	for _, program := range runningData.programs {
		chans = append(chans, program.channel)
	}
	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	for {
		potentially_runable_processes := false
		for _, program := range runningData.programs {
			switch program.programStatus {
			case PROC_FATAL:
				if runningData.allConfig.SuperVisorD.ExitOn == "ANY_FATAL" {
					log.Fatal("Exiting due to ANY_FATAL")
				}
			default:
				potentially_runable_processes = true
				break
			}
		}
		if potentially_runable_processes {
			// We will wait at this Select until one of our child processes changes state
			// and notifies us...
			chosen, value, ok := reflect.Select(cases)
			if ok {
				ch := chans[chosen]
				state := value.Interface().(ProcStatus)
				// Find the program that uses this channel, then act.
				for _, program := range runningData.programs {
					if program.channel == ch {
						program.UpdateStatus(state)
						if state == PROC_RUNNING {
							program.SetPriority()
						} else {
							// If we are supposed to start it again then do so
							program.StartRunableProcess()
						}

						break
					}
				}
			} else {
				potentially_runable_processes = false
			}
		}

		if !potentially_runable_processes {
			if runningData.allConfig.SuperVisorD.ExitOn == "ALL_FATAL" {
				log.Fatal("Exiting due to ALL_FATAL")
			}
			log.Println("Nothing to do, waiting...")
			time.Sleep(5 * time.Second)
		}

	}
}

func (prog *Program) StartRunableProcess() {
	switch prog.programStatus {
	case PROC_STOPPED:
		if prog.startable {
			log.Printf("Starting %s\n", prog.config.ProcessName)
			prog.UpdateStatus(PROC_STARTING)
			prog.startCount++
			go prog.RunSingleProcess()
		}
	case PROC_BACKOFF:
		prog.startCount++
		prog.TryRestart()
	case PROC_EXITED:
		prog.startCount = 0
		prog.TryRestart()
	}
}

func (prog *Program) TryRestart() {
	if prog.CanRestart() {
		log.Printf("Restarting %s\n", prog.config.ProcessName)
		prog.UpdateStatus(PROC_STARTING)
		go prog.RunSingleProcess()
	} else if prog.programStatus == PROC_STOPPED || prog.programStatus == PROC_EXITED {
		log.Printf("%s is %s, not restarting\n", prog.config.ProcessName, stateToString(prog.programStatus))
	} else {
		prog.UpdateStatus(PROC_FATAL)
		log.Printf("Process '%s' will not restart automatically\n", prog.config.ProcessName)
	}
}

func (program *Program) CanRestart() (bool) {
	if program.programStatus == PROC_BACKOFF && program.startCount > program.config.StartRetries {
		return false
	}

	if program.config.AutoRestart == "true" {
		return true
	}

	if program.config.AutoRestart == "unexpected" {
		exitStatus := program.exitStatus
		log.Printf("Handling 'unexpected' exit, status: %s", exitStatus)
		if strings.HasPrefix(exitStatus, "exit status ") {
			exitStatus = program.exitStatus[12:]
		}
		expectedCodes := strings.Split(program.config.ExitCodes, ",")
		for _, expectedCode := range expectedCodes {
			if expectedCode == exitStatus {
				return true
			}
		}

		log.Printf("Unexpected error (%s)\n", program.exitStatus)
		log.Printf("Expecting (%s)\n", program.config.ExitCodes)
	}

	return false
}

func (program *Program) CreateCommand() (*exec.Cmd) {
	var cmd *exec.Cmd

	if len(program.config.Command) > 1 {
		args := []string{}
		args = program.config.Command[1:]
		log.Printf("Running %s %s\n", program.commandPath, args)
		cmd = exec.Command(program.commandPath, args...)
	} else {
		log.Printf("Running %s\n", program.commandPath)
		cmd = exec.Command(program.commandPath)
	}
	program.command = cmd
	return cmd
}

func (program *Program) SetPriority() {
	cmd := program.command
	var err error
	err = syscall.Setpriority(syscall.PRIO_PROCESS, cmd.Process.Pid, program.config.Priority)
	if err == nil {
		log.Printf("PRIORITY: Process %s priority set %d",
				program.config.ProcessName, program.config.Priority)

	} else {
		log.Printf("PRIORITY: Could not set priority for process %s", program.config.ProcessName)
		log.Println(err)
	}
}

func (program *Program) SetIO() {
	// Connect stdout
	if program.config.StdoutLogfile == "" || program.config.StdoutLogfile == "AUTO" {
		program.config.StdoutLogfile = "/dev/stdout"
	}
	stdout, stdouterr := os.Create(program.config.StdoutLogfile)
	if stdouterr != nil {
		log.Printf("Could not create %s", program.config.StdoutLogfile)
	}
	program.stdout = stdout

	// Connect stderr
	if program.config.StderrLogfile == "" || program.config.StderrLogfile == "AUTO" {
		program.config.StderrLogfile = "/dev/stderr"
	}
	stderr, stderrerr := os.Create(program.config.StderrLogfile)
	if stderrerr != nil {
		log.Printf("Could not create %s", program.config.StdoutLogfile)
	}
	program.stderr = stderr

	program.command.Stdout = program.stdout
	program.command.Stderr = program.stderr
}

func (program *Program) RunSingleProcess() {
	cmd := program.CreateCommand()

	program.SetIO()
	defer program.stdout.Close()
	defer program.stderr.Close()

	runerr := cmd.Start()
	if runerr != nil {
		program.channel <- PROC_BACKOFF
		return
	}

	program.channel <- PROC_RUNNING
	exitVal := cmd.Wait()

	if exitVal != nil {
		program.exitStatus = fmt.Sprintf("%v", exitVal)
		program.channel <- PROC_BACKOFF
	} else {
		program.exitStatus = "0"
		program.channel <- PROC_EXITED
	}
}