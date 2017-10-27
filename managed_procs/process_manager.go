package managed_procs

import (
	"os/exec"
	"fmt"
	"os"
	"time"
	"strings"
	"reflect"
)

type ProcStatus int

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
	channel                chan string
	commandPath            string
	programStatusTimestamp time.Time
}

type RunningData struct {
	programs  []*Program
	allConfig AllConfig
}

func (allConfig AllConfig) RunAllProcesses()  {
	progChannelRef := make(map[string]chan string)
	for _, prog := range allConfig.Programs {
		if prog.AutoStart {
			progChan := make(chan string)
			progChannelRef[prog.ProcessName] = progChan
			go prog.ManageProcess(progChan)
		}
	}

	for {
		for processName, progChan := range progChannelRef {
			select {
			case exitStatus :=  <-progChan:
				fmt.Printf("'%s' ended with '%s'\n", processName, exitStatus)
				delete(progChannelRef, processName)
			default:
				fmt.Print(".")
				time.Sleep(1 * time.Second)
			}
		}
		if len(progChannelRef) == 0 {
			return
		}
	}
}

func (programConfig ProgramConfigSection) ManageProcess(progChan chan string) {
	path, err := exec.LookPath(programConfig.Command[0])
	if err != nil {
		fmt.Printf("Could not find command: %s\n", err)
		return
	}

	args := []string{}
	if len(programConfig.Command) > 1 {
		args = programConfig.Command[1:]
	}
	fmt.Printf("Path %s Args %s\n", path, args)
	cmd := exec.Command(path, args...)

	stdout, stdouterr := os.Create(programConfig.StdoutLogfile)
	if stdouterr != nil {
		fmt.Printf("Could not create %s", programConfig.StdoutLogfile)
	}
	defer stdout.Close()

	stderr, stderrerr := os.Create(programConfig.StderrLogfile)
	if stderrerr != nil {
		fmt.Printf("Could not create %s", programConfig.StdoutLogfile)
	}
	defer stderr.Close()

	cmd.Stdout = stdout
	cmd.Stderr = stderr
	runerr := cmd.Start()
	if runerr != nil {
		progChan <- "START_FAILURE"
		close(progChan)
	}
	exitVal := cmd.Wait()
	var exitValStatus string
	if exitVal != nil {
		exitValStatus = fmt.Sprintf("EXIT_%v", exitVal)
	} else {
		exitValStatus = "EXIT_OK"
	}

	progChan <- exitValStatus
	close(progChan)
}

func (program *Program) UpdateStatus(status ProcStatus) {
	program.programStatus = status
	program.programStatusTimestamp = time.Now()
	var programStatusVal = "unknown"
	switch program.programStatus {
	case PROC_STOPPED:
		programStatusVal = "PROC_STOPPED"
	case PROC_RUNNING:
		programStatusVal = "PROC_RUNNING"
	case PROC_STARTING:
		programStatusVal = "PROC_STARTING"
	case PROC_FATAL:
		programStatusVal = "PROC_FATAL"
	case PROC_BACKOFF:
		programStatusVal = "PROC_BACKOFF"
	case PROC_EXITED:
		programStatusVal = "PROC_EXITED"
	}
	fmt.Printf("%s changed state to %v\n", program.config.ProcessName, programStatusVal)
}

func (allConfig AllConfig) InitialiseProcesses() []*Program {
	programs := []*Program{}
	for _, programConfig := range allConfig.Programs {
		aProgram := Program{
			config:        programConfig,
			exitStatus:    "",
			startCount:    0,
			channel:       make(chan string),
		}
		aProgram.UpdateStatus(PROC_STOPPED)

		path, err := exec.LookPath(aProgram.config.Command[0])
		if err != nil {
			fmt.Printf("Could not find command: %s\n", err)
			aProgram.UpdateStatus(PROC_FATAL)
			continue
		}
		aProgram.commandPath = path

		aProgram.channel = make(chan string)

		programs = append(programs, &aProgram)
	}
	return programs
}

func (allConfig AllConfig) RunAllProcesses2() {
	runningData := RunningData{
		programs: allConfig.InitialiseProcesses(),
		allConfig: allConfig,
	}
	runningData.StartRunableProcesses()
	runningData.MonitorRunningProcesses()
}

func (runningData RunningData) MonitorRunningProcesses() {
	for {
		count := 0
		for _, program := range runningData.programs {
			select {
			case <-program.channel:
				fmt.Printf("%s channel notification\n", program.config.ProcessName)
			default:
				time.Sleep(1 * time.Second)
			}

			switch program.programStatus {
			case PROC_STOPPED:
				count++
			case PROC_STARTING:
				count++
			case PROC_RUNNING:
				count++
			default:
				runningData.StartRunableProcesses()
			}

		}
		if count == 0 {
			fmt.Println("No programs running")
			return
		}
	}
}

func (runningData RunningData) MonitorRunningProcesses2() {
	var chans []chan string
	for _, program := range runningData.programs {
		chans = append(chans, program.channel)
	}
	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}
	for {
		chosen, value, ok := reflect.Select(cases)
		if ok {
			ch := chans[chosen]
			msg := value.String()
			fmt.Printf("TEST %s\n", msg)
			for _, program := range runningData.programs {
				if program.channel == ch {

				}
			}
		}
	}
}

func (runningData RunningData) StartRunableProcesses() {
	for _, prog := range runningData.programs {
		if prog.config.AutoStart {
			switch prog.programStatus {
			case PROC_STOPPED:
				fmt.Printf("Starting %s\n", prog.config.ProcessName)
				prog.UpdateStatus(PROC_STARTING)
				prog.startCount++
				go prog.RunSingleProcess()
			case PROC_BACKOFF:
				prog.TryRestart()
			case PROC_EXITED:
				prog.TryRestart()
			}
		}
	}
}

func (prog *Program) TryRestart() {
	if prog.CanRestart() {
		fmt.Printf("Restarting %s\n", prog.config.ProcessName)
		prog.UpdateStatus(PROC_STARTING)
		prog.startCount++
		go prog.RunSingleProcess()
	} else {
		fmt.Printf("%s has failed and cannot restart\n", prog.config.ProcessName)
		prog.UpdateStatus(PROC_FATAL)
	}
}

func (program *Program) CanRestart() (bool) {
	if program.startCount >= program.config.StartRetries {
		return false
	}

	if program.config.AutoRestart == "true" {
		return true
	}

	if program.config.AutoRestart == "unexpected" {
		if strings.HasPrefix(program.exitStatus, "exit status ") {
			exitStatus := program.exitStatus[12:]
			expectedCodes := strings.Split(program.config.ExitCodes, ",")
			for _, expectedCode := range expectedCodes {
				if expectedCode == exitStatus {
					return true
				}
			}
		}
		fmt.Printf("Unexpected error (%s)\n", program.exitStatus)
		fmt.Printf("Expecting (%s)\n", program.config.ExitCodes)
	}

	return false
}

func (program *Program) RunSingleProcess() {
	var cmd *exec.Cmd

	if len(program.config.Command) > 1 {
		args := []string{}
		args = program.config.Command[1:]
		fmt.Printf("Running %s %s\n", program.commandPath, args)
		cmd = exec.Command(program.commandPath, args...)
	} else {
		fmt.Printf("Running %s\n", program.commandPath)
		cmd = exec.Command(program.commandPath)
	}

	// Connect stdout
	if program.config.StdoutLogfile == "" || program.config.StdoutLogfile == "AUTO" {
		program.config.StdoutLogfile = "/dev/stdout"
	}
	stdout, stdouterr := os.Create(program.config.StdoutLogfile)
	if stdouterr != nil {
		fmt.Printf("Could not create %s", program.config.StdoutLogfile)
	}
	defer stdout.Close()
	program.stdout = stdout

	// Connect stderr
	if program.config.StderrLogfile == "" || program.config.StderrLogfile == "AUTO" {
		program.config.StderrLogfile = "/dev/stderr"
	}
	stderr, stderrerr := os.Create(program.config.StderrLogfile)
	if stderrerr != nil {
		fmt.Printf("Could not create %s", program.config.StdoutLogfile)
	}
	defer stderr.Close()
	program.stderr = stderr

	cmd.Stdout = program.stdout
	cmd.Stderr = program.stderr
	runerr := cmd.Start()
	if runerr != nil {
		program.UpdateStatus(PROC_BACKOFF)
		program.channel <- "changed"
		return
	}

	program.UpdateStatus(PROC_RUNNING)
	program.channel <- "changed"
	exitVal := cmd.Wait()

	if exitVal != nil {
		program.exitStatus = fmt.Sprintf("%v", exitVal)
		program.UpdateStatus(PROC_BACKOFF)
		program.channel <- "changed"
	} else {
		program.exitStatus = "0"
		program.UpdateStatus(PROC_EXITED)
		program.channel <- "changed"
	}
}