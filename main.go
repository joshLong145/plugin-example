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
	"path/filepath"
	"runtime"

	hplugin "github.com/hashicorp/go-plugin"

	"github.com/ignite/cli/ignite/services/chain"
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
	gob.Register(plugin.Manifest{})
	gob.Register(plugin.ExecutedCommand{})
	gob.Register(plugin.ExecutedHook{})
}

type p struct{}

func (p) Manifest() (plugin.Manifest, error) {
	return plugin.Manifest{
		Name: "example-plugin",
		Commands: []plugin.Command{
			{
				Use:               "ipfs",
				PlaceCommandUnder: "ignite",
				Commands: []plugin.Command{
					{Use: "shutdown"},
					{Use: "restart"},
				},
			},
		},
		Hooks: []plugin.Hook{
			{
				Name:        "serve",
				PlaceHookOn: "ignite chain serve",
			},
			{
				Name:        "build",
				PlaceHookOn: "ignite chain build",
			},
		},
	}, nil
}

func (p) Execute(cmd plugin.ExecutedCommand) error {
	// According to the number of declared commands, you may need a switch:
	switch cmd.Use {
	case "shutdown":
		fmt.Printf("Killing ipfs daemon")
		err := ipfs_process.Process.Kill()
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		fmt.Println("done killing ipfs daemon")
	case "restart":
		fmt.Println("Restarting ipfs host")
		if ipfs_process != nil {
			err := ipfs_process.Process.Kill()
			if err != nil {
				return err
			}
		}

		ipfs_process = exec.Command("kubo/ipfs", "daemon")
		err := ipfs_process.Start()
		if err != nil {
			return err
		}

	}
	return nil
}

func (p) ExecuteHookPre(hook plugin.ExecutedHook) error {
	fmt.Printf("hook %s", hook.Name)
	switch hook.Name {
	case "build":
		err := resolveFiles()
		if err != nil {
			return err
		}
		fmt.Println(`Kubo IPFS has been resolved to "kubo/"`)
	case "serve":
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
		return fmt.Errorf("hook not defined %s", hook.Name)
	}

	return nil
}

func (p) ExecuteHookPost(hook plugin.ExecutedHook) error {
	switch hook.Hook.Name {
	case "build":
	case "serve":
		fmt.Printf(`post event triggered for: %s\n`, hook.Name)
	default:
		return fmt.Errorf("hook not defined")
	}

	return nil
}

func (p) ExecuteHookCleanUp(hook plugin.ExecutedHook) error {
	switch hook.Hook.Name {
	case "build":
	case "serve":
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
				"ExtractTarGz: uknown type: %d in %s",
				header.Typeflag,
				header.Name)
		}

	}

	return nil
}

func getChain(cmd plugin.ExecutedCommand, chainOption ...chain.Option) (*chain.Chain, error) {
	var (
		home, _ = cmd.Flags().GetString("home")
		path, _ = cmd.Flags().GetString("path")
	)
	if home != "" {
		chainOption = append(chainOption, chain.HomePath(home))
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return chain.New(absPath, chainOption...)
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
