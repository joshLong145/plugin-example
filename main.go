package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	hplugin "github.com/hashicorp/go-plugin"

	"github.com/ignite/cli/ignite/services/plugin"
)

const (
	DARWIN_ARM64_DOWNLOAD = "https://dist.ipfs.tech/kubo/v0.16.0/kubo_v0.16.0_darwin-arm64.tar.gz"
	DARWIN_AMD64_DOWNLOAD = "https://dist.ipfs.tech/kubo/v0.16.0/kubo_v0.16.0_darwin-amd64.tar.gz"
	LINUX_ARM64_DOWNLOAD  = "https://dist.ipfs.tech/kubo/v0.16.0/kubo_v0.16.0_linux-arm64.tar.gz"
	LINUX_AMD64_DOWNLOAD  = "https://dist.ipfs.tech/kubo/v0.16.0/kubo_v0.16.0_linux-amd64.tar.gz"
)

var (
	ipfs_process *exec.Cmd
)

func init() {
	gob.Register(plugin.Command{})
	gob.Register(plugin.Hook{})
}

type p struct{}

func (p) Commands() []plugin.Command {
	// TODO: write your command list here
	return []plugin.Command{
		{
			Use:               "ipfs",
			PlaceCommandUnder: "ignite",
			Commands: []plugin.Command{
				{Use: "shutdown"},
			},
		},
	}
}

func (p) Hooks() []plugin.Hook {
	return []plugin.Hook{
		{
			Name:        "my-hook-serve",
			PlaceHookOn: "ignite chain serve",
		},
		{
			Name:        "my-hook-build",
			PlaceHookOn: "ignite chain build",
		},
	}
}

func (p) Execute(cmd plugin.Command, args []string) error {
	// According to the number of declared commands, you may need a switch:
	switch cmd.Use {
	case "shutdown":
		fmt.Printf("Killing ipfs daemon")
		_, err := exec.Command("kubo/ipfs", "shutdown").Output()
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		fmt.Println("done killing ipfs daemon")
	}
	return nil
}

func (p) ExecuteHookPre(hook plugin.Hook, args []string) error {
	switch hook.Name {
	case "my-hook-build":
		err := resolveFiles()
		if err != nil {
			return err
		}
		fmt.Println(`Kubo IPFS has been resolved to "kubo/"`)
	case "my-hook-serve":
		err := resolveFiles()
		if err != nil {
			return err
		}

		fmt.Println("Starting ipfs")
		ipfs_process = exec.Command("kubo/ipfs", "daemon")
		err = ipfs_process.Start()
		if err != nil {
			return err
		}

		fmt.Printf("daemon pid %d\n", ipfs_process.Process.Pid)
	default:
		return fmt.Errorf("hook not defined")
	}

	return nil
}

func (p) ExecuteHookPost(hook plugin.Hook, args []string) error {
	switch hook.Name {
	case "my-hook-build":
	case "my-hook-serve":
		fmt.Printf(`post event triggered for: %s\n`, hook.Name)
	default:
		return fmt.Errorf("hook not defined")
	}

	return nil
}

func (p) ExecuteHookCleanUp(hook plugin.Hook, args []string) error {
	switch hook.Name {
	case "my-hook-build":
	case "my-hook-serve":
		fmt.Println("Cleaning Up tmp directory")
		err := os.Remove("tmp/ipfs.tar")
		if err != nil {
			return err
		}
		os.Remove("tmp/")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("hook not defined")
	}

	return nil
}

func resolveFiles() error {
	if _, err := os.Stat("kubo"); err != nil {
		fmt.Println("downloading ipfs exeutable ....")
		err := os.Mkdir("tmp", 0777)
		if err != nil {
			return err
		}
		ipfs, err := os.Create("tmp/ipfs.tar")
		ipfs.Chmod(0777)

		if err != nil {
			return err
		}
		var resp *http.Response
		var uri string
		switch runtime.GOOS {
		case "linux":
			switch runtime.GOARCH {
			case "amd64":
				uri = LINUX_AMD64_DOWNLOAD
			case "arm64":
				uri = LINUX_ARM64_DOWNLOAD
			}
		case "darwin":
			switch runtime.GOARCH {
			case "amd64":
				uri = DARWIN_AMD64_DOWNLOAD
			case "arm64":
				uri = DARWIN_ARM64_DOWNLOAD
			}
		}

		resp, err = http.Get(uri)
		if err != nil {
			return err
		}
		_, err = io.Copy(ipfs, resp.Body)
		if err != nil {
			return err
		}
		fmt.Println("Done downloading ipfs")
		fmt.Println("unzipping ...")
		r, err := os.Open("tmp/ipfs.tar")
		if err != nil {
			return err
		}
		err = extractTar(r)
		if err != nil {
			return err
		}
		os.Chmod("kubo/ipfs", 0777)
	}

	return nil
}

func extractTar(stream io.Reader) error {
	uncompressedStream, err := gzip.NewReader(stream)
	if err != nil {
		log.Fatal("ExtractTarGz: NewReader failed")
	}

	tarReader := tar.NewReader(uncompressedStream)

	for true {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("ExtractTarGz: Next() failed: %s", err.Error())
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(header.Name, 0755); err != nil {
				log.Fatalf("ExtractTarGz: Mkdir() failed: %s", err.Error())
			}
		case tar.TypeReg:
			outFile, err := os.Create(header.Name)
			if err != nil {
				log.Fatalf("ExtractTarGz: Create() failed: %s", err.Error())
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				log.Fatalf("ExtractTarGz: Copy() failed: %s", err.Error())
			}
			outFile.Close()

		default:
			log.Fatalf(
				"ExtractTarGz: uknown type: %s in %s",
				header.Typeflag,
				header.Name)
		}

	}

	return nil
}

func main() {
	var pluginMap = map[string]hplugin.Plugin{
		"example-plugin": &plugin.InterfacePlugin{Impl: &p{}},
	}

	hplugin.Serve(&hplugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig(),
		Plugins:         pluginMap,
	})
}
