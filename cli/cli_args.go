package cli

import (
	"aws-sagemaker-edge-quick-device-setup/constants"
	"aws-sagemaker-edge-quick-device-setup/distinfo"
	"flag"
	"path/filepath"
	"fmt"
	"log"
	"os"
	"strings"
)

type TargetPlatform struct {
	Os          string
	Arch        string
	Accelerator string
}

func (tp *TargetPlatform) Print() {
	fmt.Println("Target Platform")
	fmt.Printf("\tOs: %s\n", tp.Os)
	fmt.Printf("\tArchitecture: %s\n", tp.Arch)
	fmt.Printf("\tAccelerator: %s\n", tp.Accelerator)
}

func (tp *TargetPlatform) Validate() {
	if tp.Os != "linux" && tp.Os != "windows" {
		log.Fatal("Invalid Os!")
	}

	if tp.Os == "linux" {
		if tp.Arch != constants.ARM64 && tp.Arch != constants.ARMV8 && tp.Arch != constants.AMD64 && tp.Arch != constants.X64 && tp.Arch != constants.X86_64 {
			log.Fatal("Invalid architecture for Linux.")
		}
	}
	if tp.Os == "windows" {
		if tp.Arch != constants.AMD64 && tp.Arch != constants.I386 && tp.Arch != constants.X86 && tp.Arch != constants.X64 && tp.Arch != constants.X86_64 {
			log.Fatal("Invalid architecture for Windows.")
		}
	}
}

type CliArgs struct {
	DeviceFleet       string
	DeviceName        string
	IotThingType      string
	IotThingName      string
	DeviceFleetRole   string
	DeviceFleetBucket string
	Account           string
	Region            string
	AgentDirectory    string
	S3FolderPrefix    string
	TargetPlatform    TargetPlatform
}

func (cliArgs *CliArgs) Print() {
	fmt.Printf("Account: %s\n", cliArgs.Account)
	fmt.Printf("Region: %s\n", cliArgs.Region)
	fmt.Printf("DeviceFleet: %s\n", cliArgs.DeviceFleet)
	fmt.Printf("DeviceName: %s\n", cliArgs.DeviceName)
	fmt.Printf("IOT Thing Type: %s\n", cliArgs.IotThingType)
	fmt.Printf("IOT Thing Name: %s\n", cliArgs.IotThingName)
	fmt.Printf("Device Fleet Role: %s\n", cliArgs.DeviceFleetRole)
	fmt.Printf("Device Fleet Bucket: %s\n", cliArgs.DeviceFleetBucket)
	fmt.Printf("Agent Directory: %s\n", cliArgs.AgentDirectory)
	cliArgs.TargetPlatform.Print()
}

func ParseArgs(cliArgs *CliArgs) {
	accountId := flag.String("account", "", "AWS AccountId (required).")
	region := flag.String("region", "us-west-2", "AWS Region.")
	deviceFleet := flag.String("deviceFleet", "", "Name of the device fleet (required).")
	deviceName := flag.String("deviceName", "", "Name of the device (required).")

	targetOs := flag.String("os", "", "Name of operating system (optional with distribution binary).")
	targetArch := flag.String("arch", "", "Name of device architecture (optional with distribution binary).")
	targetAccelerator := flag.String("accelerator", "", "Name of accelerator (optional).")

	iotThingType := flag.String("iotThingType", "", "Iot thing type for the device (optional/autogenerated).")
	iotThingName := flag.String("iotThingName", "", "IOT thing name for the device (optional/autogenerated).")
	deviceFleetRole := flag.String("deviceFleetRole", "", "Name of the role for the device fleet (optional/autogenerated).")
	deviceFleetBucket := flag.String("deviceFleetBucket", "", "Bucket to store device related data (optional/autogenerated).")
	s3FolderPrefix := flag.String("s3FolderPrefix", "", "S3 prefix to store captured data (optional/autogenerated).")

	cwd, err := os.Getwd()

	if err != nil {
		log.Fatal("Error ", err)
	}

	defaultAgentDirectory := filepath.Join(cwd, "demo-agent")
	agentDirectory := flag.String("agentDirectory", defaultAgentDirectory, "Local path to store agent")

	version := flag.Bool("version", false, "Print the version of aws-sagemaker-edge-quick-device-setup")
	dist := flag.Bool("dist", false, "Print distribution information.")

	flag.Parse()

	if *version {
		fmt.Println(distinfo.VERSION)
		os.Exit(0)
	}

	fmt.Println("Distribution Information")
	fmt.Println("Version: ", distinfo.VERSION)
	if distinfo.OS != "" {
		fmt.Println("Os: ", distinfo.OS)
	}
	if distinfo.ARCH != "" {
		fmt.Println("Architecture: ", distinfo.ARCH)
	}

	if *dist {
		os.Exit(0)
	}

	if *deviceFleet == "" || *deviceName == "" || *accountId == "" {
		log.Fatal("Missing deviceFleet or deviceName or account")
	}

	cliArgs.DeviceFleet = *deviceFleet
	cliArgs.DeviceName = *deviceName

	if *targetOs == "" {
		log.Println(distinfo.OS)
		*targetOs = distinfo.OS
	}

	if *targetArch == "" {
		*targetArch = distinfo.ARCH
	}

	cliArgs.TargetPlatform = TargetPlatform{Os: strings.ToLower(*targetOs), Arch: strings.ToLower(*targetArch), Accelerator: strings.ToLower(*targetAccelerator)}
	cliArgs.TargetPlatform.Validate()

	cliArgs.Account = *accountId
	cliArgs.Region = strings.ToLower(*region)
	cliArgs.AgentDirectory = *agentDirectory

	if *iotThingType == "" {
		*iotThingType = fmt.Sprintf("Sagemaker_%s", cliArgs.DeviceFleet)
	}
	if *iotThingName == "" {
		*iotThingType = fmt.Sprintf("Sagemaker_%s", cliArgs.DeviceName)
	}
	if *deviceFleetRole == "" {
		*deviceFleetRole = fmt.Sprintf("Sagemaker_%s_role", cliArgs.DeviceFleet)
	}

	if *s3FolderPrefix == "" {
		*s3FolderPrefix = "demo"
	}

	cliArgs.IotThingType = *iotThingType
	cliArgs.IotThingName = *iotThingType
	cliArgs.DeviceFleetRole = *deviceFleetRole
	cliArgs.DeviceFleetBucket = *deviceFleetBucket
	cliArgs.S3FolderPrefix = *s3FolderPrefix
}
