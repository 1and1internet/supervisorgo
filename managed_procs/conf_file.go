package managed_procs

import (
	"fmt"
	"gopkg.in/go-ini/ini.v1"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type SuperConfigSection struct {
	LogFile         string
	LogFileMaxBytes string
	LogFileBackups  int
	LogLevel        string
	PidFile         string
	Umask           string
	Nodaemon        bool
	Minfds          int
	MinProcs		int
	Nocleanup       bool
	ChildLogDir     string
	User            string
	Directory       string
	StripAnsi       bool
	Environment     string
	Identifier      string
	ExitOn			string
}

type EventListenerConfigSection struct {
	BufferSize    string
	Events        string
	ResultHandler string
	ProgramData   ProgramConfigSection
}

type ProgramConfigSection struct {
	Command               []string
	ProcessName           string
	NumProcs              int
	NumProcsStart         int
	Priority              int
	AutoStart             bool
	StartSecs             int
	StartRetries          int
	AutoRestart           string
	ExitCodes             string
	StopSignal            string
	StopWaitSecs          int
	StopAsGroup           bool
	KillAsGroup           bool
	User                  string
	RedirectStdErr        bool
	StdoutLogfile         string
	StdoutLogfileMaxbytes string
	StdoutLogfileBackups  int
	StdoutCaptureMaxbytes string
	StdoutEventsEnabled   bool
	StderrLogfile         string
	StderrLogfileMaxbytes string
	StderrLogfileBackups  int
	StderrCaptureMaxbytes string
	StderrEventsEnabled   bool
	Environment           string
	Directory             string
	Umask                 string
	ServerUrl             string
}

type AllConfig struct {
	SuperVisorD SuperConfigSection
	EventListeners map[string]EventListenerConfigSection
	Programs map[string]ProgramConfigSection
}

func get_config_file(supervisorConf string) (string) {
	possibleConfigFiles := []string{
		supervisorConf,
	}

	pwd, ok := os.LookupEnv("PWD")
	if ok {
		possibleConfigFiles = append(
			possibleConfigFiles,
			fmt.Sprintf("%s/supervisord.conf", pwd),
			fmt.Sprintf("%s/etc/supervisord.conf", pwd),
		)
	}

	possibleConfigFiles = append(
		possibleConfigFiles,
		"/etc/supervisor/supervisord.conf",
		"../etc/supervisord.conf",
		"../supervisord.conf",
	)

	// Get the first valid config file, using the
	// one specified using the -c flag first...
	for _, configFile := range possibleConfigFiles {
		_, err := os.Stat(configFile)
		if err == nil {
			return configFile
		}
		fmt.Printf("%s %s\n", configFile, err)
	}

	return ""
}

func LoadAllConfig(supervisorConf *string) (AllConfig) {
	allConfig := AllConfig{}
	superConfigFile := get_config_file(*supervisorConf)
	allConfig.EventListeners = make(map[string]EventListenerConfigSection)
	allConfig.Programs = make(map[string]ProgramConfigSection)

	iniConfig, err := ini.Load(superConfigFile)
	if err != nil {
		fmt.Printf("Issue opening ini: [%s] [%v]", superConfigFile, err)
		return allConfig
	}

	for _, sectionName := range iniConfig.SectionStrings() {
		//fmt.Printf("Section: %s\n", sectionName)
		section, _ := iniConfig.GetSection(sectionName)
		if sectionName == "supervisord" {
			allConfig.LoadSuperConfig(section)
		} else {
			allConfig.HandleOtherConfigSections(section, sectionName)
		}
	}

	return allConfig
}

func (allConfig *AllConfig) HandleOtherConfigSections(iniSection *ini.Section, sectionName string) {
	if strings.HasPrefix(sectionName, "eventlistener") {
		name := strings.Split(sectionName, ":")[1]
		_, ok := allConfig.EventListeners[name]
		if !ok {
			allConfig.LoadEventListener(iniSection, name)
		} else {
			fmt.Printf("Section %s is duplicated. Ignoring extra(s).\n", sectionName)
		}
	} else if strings.HasPrefix(sectionName, "program") {
		name := strings.Split(sectionName, ":")[1]
		_, ok := allConfig.Programs[name]
		if !ok {
			programSection := GetDefaultProgramSection(name)
			programSection.LoadProgram(iniSection, name)
			allConfig.Programs[name] = programSection
		} else {
			fmt.Printf("Section %s is duplicated. Ignoring extra(s).\n", sectionName)
		}
	} else if sectionName == "include" {
		if iniSection.HasKey("files") {
			fileglobs := strings.Split(iniSection.Key("files").String(), " ")
			for _, fileglob := range fileglobs {
				files, err := filepath.Glob(fileglob)
				if err == nil {
					for _, file := range files {
						//fmt.Printf("Loading %s\n", file)
						includedIniConfig, err2 := ini.Load(file)
						if err2 != nil {
							fmt.Printf("Issue opening ini(2): [%s] [%v]\n", file, err)
						} else {
							for _, sectionName := range includedIniConfig.SectionStrings() {
								//fmt.Printf("Section: %s\n", sectionName)
								section, _ := includedIniConfig.GetSection(sectionName)
								if sectionName == "supervisord" {
									fmt.Printf("Ignoring supervisord section in %s\n", file)
								} else {
									allConfig.HandleOtherConfigSections(section, sectionName)
								}
							}
						}
					}
				} else {
					fmt.Printf("Fileglob error: %s\n", err)
				}
			}
		}
	}
}

func (allConfig *AllConfig) LoadSuperConfig(section *ini.Section) {
	var err error
	for _, key := range section.KeyStrings() {
		//fmt.Printf("		%s = %v\n", key, section.Key(key))

		if key == "logfile" {
			allConfig.SuperVisorD.LogFile = section.Key(key).String()
		} else if key == "logfile_maxbytes" {
			allConfig.SuperVisorD.LogFileMaxBytes = section.Key(key).String()
		} else if key == "logfile_backups" {
			allConfig.SuperVisorD.LogFileBackups, err = section.Key(key).Int()
		} else if key == "loglevel" {
			allConfig.SuperVisorD.LogLevel = section.Key(key).String()
		} else if key == "pidfile" {
			allConfig.SuperVisorD.PidFile = section.Key(key).String()
		} else if key == "umask" {
			allConfig.SuperVisorD.Umask = section.Key(key).String()
		} else if key == "nodaemon" {
			allConfig.SuperVisorD.Nodaemon, err = section.Key(key).Bool()
		} else if key == "minfds" {
			allConfig.SuperVisorD.Minfds, err = section.Key(key).Int()
		} else if key == "minprocs" {
			allConfig.SuperVisorD.MinProcs, err = section.Key(key).Int()
		} else if key == "nocleanup" {
			allConfig.SuperVisorD.Nocleanup, err = section.Key(key).Bool()
		} else if key == "childlogdir" {
			allConfig.SuperVisorD.ChildLogDir = section.Key(key).String()
		} else if key == "user" {
			allConfig.SuperVisorD.User = section.Key(key).String()
		} else if key == "directory" {
			allConfig.SuperVisorD.Directory = section.Key(key).String()
		} else if key == "strip_ansi" {
			allConfig.SuperVisorD.StripAnsi, err = section.Key(key).Bool()
		} else if key == "environment" {
			allConfig.SuperVisorD.Environment = section.Key(key).String()
		} else if key == "identifier" {
			allConfig.SuperVisorD.Identifier = section.Key(key).String()
		} else if key == "exit_on" {
			allConfig.SuperVisorD.ExitOn = section.Key(key).String()
		}

		if err != nil {
			fmt.Printf("WARNING: Trouble converting that key\n")
			err = nil
		}
	}
}

func GetDefaultProgramSection(name string) (ProgramConfigSection) {
	// set defaults
	programSection := ProgramConfigSection{
		ProcessName: name,
		NumProcs: 1,
		NumProcsStart: 0,
		Priority: 999,
		AutoStart: true,
		StartSecs: 1,
		StartRetries: 3,
		AutoRestart: "unexpected",
		ExitCodes: "0,2",
		StopSignal: "TERM",
		StopWaitSecs: 10,
		StopAsGroup: false,
		KillAsGroup: false,
		User: "",
		RedirectStdErr: false,
		StdoutLogfile: "AUTO",
		StdoutLogfileMaxbytes: "50MB",
		StdoutLogfileBackups: 10,
		StdoutCaptureMaxbytes: "50MB",
		StdoutEventsEnabled: false,
		StderrLogfile: "AUTO",
		StderrLogfileMaxbytes: "50MB",
		StderrLogfileBackups: 10,
		StderrEventsEnabled: false,
		Environment: "",
		Directory: "",
		Umask: "",
		ServerUrl: "AUTO",
	}
	return programSection
}

func replaceCommandEnvars (origString string) string {
	re := regexp.MustCompile(`%\((.*?)\)s`)
	return re.ReplaceAllStringFunc(origString, stripAndGetEnv)
}

func stripAndGetEnv(toreplace string) string {
	re := regexp.MustCompile(`%\((.*?)\)s`)
	toreplace = re.ReplaceAllString(toreplace, "${1}")
	return os.Getenv(toreplace)
}

func (configFileSection *ProgramConfigSection) LoadProgram(section *ini.Section, name string) {
	var err error
	for _, key := range section.KeyStrings() {
		//fmt.Printf("		%s = %v\n", key, section.Key(key))

		// If something is quoted, the quotes should be stripped and the content of the quotes should become 1 arg.
		// i.e. bash -c "source stuff && do/otherStuff.sh"
		// becomes ["bash", "-c", "source stuff && do/otherStuff.sh"], not
		// becomes ["bash", "-c", "\"source stuff && do/otherStuff.sh\""]

		if key == "command" {
			value := *section.Key(key)
			commandParts := strings.Split(replaceCommandEnvars(value.Value()), " ")
			var realCommandParts []string
			inQuotes := false
			quoted := ""
			for _, commandPart := range commandParts {
				if commandPart == "" {
					continue
				}
				if ! inQuotes && commandPart[0] == '"' {
					inQuotes = true
					quoted = commandPart[1:]
				} else if inQuotes && commandPart[len(commandPart)-1] != '"' {
					quoted = fmt.Sprintf("%s %s", quoted, commandPart)
				} else if inQuotes {
					inQuotes = false
					quoted = fmt.Sprintf("%s %s", quoted, commandPart[:len(commandPart)-1])
					realCommandParts = append(realCommandParts, quoted)
				} else {
					realCommandParts = append(realCommandParts, commandPart)
				}
			}
			configFileSection.Command = append(configFileSection.Command, realCommandParts...)
		} else if key == "process_name" {
			configFileSection.ProcessName = section.Key(key).String()
		} else if key == "numprocs" {
			configFileSection.NumProcs, err = section.Key(key).Int()
		} else if key == "numprocs_start" {
			configFileSection.NumProcsStart, err = section.Key(key).Int()
		} else if key == "priority" {
			configFileSection.Priority, err = section.Key(key).Int()
		} else if key == "autostart" {
			configFileSection.AutoStart, err = section.Key(key).Bool()
		} else if key == "startsecs" {
			configFileSection.StartSecs, err = section.Key(key).Int()
		} else if key == "startretries" {
			configFileSection.StartRetries, err = section.Key(key).Int()
		} else if key == "autorestart" {
			configFileSection.AutoRestart = section.Key(key).String()
		} else if key == "exitcodes" {
			configFileSection.ExitCodes = section.Key(key).String()
		} else if key == "stopsignal" {
			configFileSection.StopSignal = section.Key(key).String()
		} else if key == "stopwaitsecs" {
			configFileSection.StopWaitSecs, err = section.Key(key).Int()
		} else if key == "stopasgroup" {
			configFileSection.StopAsGroup, err = section.Key(key).Bool()
		} else if key == "killasgroup" {
			configFileSection.KillAsGroup, err = section.Key(key).Bool()
		} else if key == "user" {
			configFileSection.User = section.Key(key).String()
		} else if key == "redirect_stderr" {
			configFileSection.RedirectStdErr, err = section.Key(key).Bool()
		} else if key == "stdout_logfile" {
			configFileSection.StdoutLogfile = section.Key(key).String()
		} else if key == "stdout_logfile_maxbytes" {
			configFileSection.StdoutLogfileMaxbytes = section.Key(key).String()
		} else if key == "stdout_logfile_backups" {
			configFileSection.StdoutLogfileBackups, err = section.Key(key).Int()
		} else if key == "stdout_capture_maxbytes" {
			configFileSection.StdoutCaptureMaxbytes = section.Key(key).String()
		} else if key == "stdout_events_enabled" {
			configFileSection.StdoutEventsEnabled, err = section.Key(key).Bool()
		} else if key == "stderr_logfile" {
			configFileSection.StderrLogfile = section.Key(key).String()
		} else if key == "stderr_logfile_maxbytes" {
			configFileSection.StderrLogfileMaxbytes = section.Key(key).String()
		} else if key == "stderr_logfile_backups" {
			configFileSection.StderrLogfileBackups, err = section.Key(key).Int()
		} else if key == "stderr_capture_maxbytes" {
			configFileSection.StderrCaptureMaxbytes = section.Key(key).String()
		} else if key == "stderr_events_enabled" {
			configFileSection.StderrEventsEnabled, err = section.Key(key).Bool()
		} else if key == "environment" {
			configFileSection.Environment = section.Key(key).String()
		} else if key == "directory" {
			configFileSection.Directory = section.Key(key).String()
		} else if key == "umask" {
			configFileSection.Umask = section.Key(key).String()
		} else if key == "serverurl" {
			configFileSection.ServerUrl = section.Key(key).String()
		}

		if err != nil {
			fmt.Printf("WARNING: Trouble converting that key\n")
			err = nil
		}

	}
}

func (allConfig *AllConfig) LoadEventListener(section *ini.Section, name string) {
	var err error

	programSection := GetDefaultProgramSection(name)
	programSection.LoadProgram(section, name)
	// The event listener is another program to run...
	allConfig.Programs[name] = programSection

	eventListenerSection := EventListenerConfigSection{
		BufferSize: "",
		Events: "",
		ResultHandler: "",
		ProgramData: programSection,
	}

	for _, key := range section.KeyStrings() {
		//fmt.Printf("		%s = %v\n", key, section.Key(key))

		if key == "buffer_size" {
			eventListenerSection.BufferSize = section.Key(key).String()
		} else if key == "events" {
			eventListenerSection.Events = section.Key(key).String()
		} else if key == "result_handler" {
			eventListenerSection.ResultHandler = section.Key(key).String()
		}

		if err != nil {
			fmt.Printf("WARNING: Trouble converting that key\n")
			err = nil
		}

	}
	allConfig.EventListeners[name] = eventListenerSection
}

func (configFileSection *ProgramConfigSection) GetEnvarMap() map[string]string {
	envarmap := make(map[string]string)
	for _, envar := range strings.Split(configFileSection.Environment, ",") {
		envar_keyval := strings.Split(envar, "=")
		if len(envar_keyval) == 2 {
			key := envar_keyval[0]
			val := envar_keyval[1]
			if len(val) > 1 && strings.HasPrefix(val, "\"")  && strings.HasSuffix(val, "\"") {
				val = strings.TrimPrefix(val, "\"")
				val = strings.TrimSuffix(val, "\"")
			}
			envarmap[key] = val
		}
	}
	return envarmap
}