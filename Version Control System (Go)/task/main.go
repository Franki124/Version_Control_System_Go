package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	vcsDir     = "./vcs"
	commitsDir = "./vcs/commits"
	configFile = "./vcs/config.txt"
	indexFile  = "./vcs/index.txt"
	logFile    = "./vcs/log.txt"
)

func main() {
	ensureVcsDir()

	if len(os.Args) < 2 || os.Args[1] == "--help" {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "config":
		handleConfig(os.Args[2:])
	case "add":
		handleAdd(os.Args[2:])
	case "commit":
		handleCommit(os.Args[2:])
	case "log":
		handleLog()
	case "checkout":
		handleCheckout(os.Args[2:])
	default:
		fmt.Printf("'%s' is not a SVCS command.\n", os.Args[1])
	}
}

func printHelp() {
	fmt.Println("These are SVCS commands:")
	fmt.Println("config     Get and set a username.")
	fmt.Println("add        Add a file to the index.")
	fmt.Println("log        Show commit logs.")
	fmt.Println("commit     Save changes.")
	fmt.Println("checkout   Restore a file.")
}

func handleConfig(args []string) {
	if len(args) == 0 {
		if content, err := ioutil.ReadFile(configFile); err == nil && len(content) > 0 {
			fmt.Printf("The username is %s.\n", string(content))
		} else {
			fmt.Println("Please, tell me who you are.")
		}
	} else {
		username := args[0]
		if err := ioutil.WriteFile(configFile, []byte(username), 0644); err != nil {
			fmt.Println("Error: Could not save username.")
		} else {
			fmt.Printf("The username is %s.\n", username)
		}
	}
}

func handleAdd(args []string) {
	if len(args) == 0 {
		if content, err := ioutil.ReadFile(indexFile); err == nil && len(content) > 0 {
			fmt.Println("Tracked files:")
			fmt.Print(string(content))
		} else {
			fmt.Println("Add a file to the index.")
		}
	} else {
		fileName := args[0]
		if _, err := os.Stat(fileName); os.IsNotExist(err) {
			fmt.Printf("Can't find '%s'.\n", fileName)
		} else {
			f, err := os.OpenFile(indexFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				fmt.Println("Error: Could not open index file.")
				return
			}
			defer f.Close()
			_, err = f.WriteString(fileName + "\n")
			if err != nil {
				fmt.Println("Error: Could not write to index file.")
			} else {
				fmt.Printf("The file '%s' is tracked.\n", fileName)
			}
		}
	}
}

func handleCommit(args []string) {
	if len(args) == 0 {
		fmt.Println("Message was not passed.")
		return
	}

	message := args[0]
	author, err := ioutil.ReadFile(configFile)
	if err != nil || len(author) == 0 {
		fmt.Println("Error: Could not read username from config.txt.")
		return
	}

	newHash, err := calculateFilesHash()
	if err != nil {
		fmt.Println("Error: Could not calculate hash.")
		return
	}

	lastCommitHash, err := getLastCommitHash()
	if err == nil && newHash == lastCommitHash {
		fmt.Println("Nothing to commit.")
		return
	}

	commitID := newHash[:6]
	commitPath := filepath.Join(commitsDir, commitID)
	if err := os.MkdirAll(commitPath, os.ModePerm); err != nil {
		fmt.Println("Error: Could not create commit directory.")
		return
	}

	files, err := getTrackedFiles()
	if err != nil {
		fmt.Println("Error: Could not read tracked files.")
		return
	}
	for _, file := range files {
		src := file
		dst := filepath.Join(commitPath, filepath.Base(file))
		if err := copyFile(src, dst); err != nil {
			fmt.Printf("Error: Could not copy '%s'.\n", file)
			return
		}
	}

	logEntry := fmt.Sprintf("commit %s\nAuthor: %s\n%s", commitID, author, message)
	if err := appendToFile(logFile, logEntry+"\n\n"); err != nil {
		fmt.Println("Error: Could not write to log file.")
		return
	}

	fmt.Println("Changes are committed.")
}

func handleLog() {
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Println("No commits yet.")
		return
	}

	content, err := ioutil.ReadFile(logFile)
	if err != nil || len(content) == 0 {
		fmt.Println("No commits yet.")
	} else {
		entries := strings.Split(strings.TrimSpace(string(content)), "\n\n")
		for i := len(entries) - 1; i >= 0; i-- {
			fmt.Print(entries[i])
			if i > 0 {
				fmt.Print("\n\n")
			}
		}
	}
}

func handleCheckout(args []string) {
	if len(args) == 0 {
		fmt.Println("Commit id was not passed.")
		return
	}

	commitID := args[0]
	commitPath := filepath.Join(commitsDir, commitID)
	if _, err := os.Stat(commitPath); os.IsNotExist(err) {
		fmt.Println("Commit does not exist.")
		return
	}

	files, err := os.ReadDir(commitPath)
	if err != nil {
		fmt.Println("Error: Could not read commit directory.")
		return
	}

	for _, file := range files {
		src := filepath.Join(commitPath, file.Name())
		dst := file.Name()
		if err := copyFile(src, dst); err != nil {
			fmt.Printf("Error: Could not restore '%s'.\n", dst)
			return
		}
	}

	fmt.Printf("Switched to commit %s.\n", commitID)
}

func ensureVcsDir() {
	err := os.MkdirAll(commitsDir, os.ModePerm)
	if err != nil {
		fmt.Println("Error: Could not create vcs directory structure.")
		return
	}

	files := []string{configFile, indexFile, logFile}
	for _, file := range files {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			fmt.Printf("Error: Could not create %s file.\n", file)
		} else {
			f.Close()
		}
	}
}

func calculateFilesHash() (string, error) {
	hasher := sha256.New()
	files, err := getTrackedFiles()
	if err != nil {
		return "", err
	}
	for _, file := range files {
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return "", err
		}
		hasher.Write(data)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func getLastCommitHash() (string, error) {
	entries, err := os.ReadDir(commitsDir)
	if err != nil {
		return "", err
	}

	if len(entries) == 0 {
		return "", fmt.Errorf("no commits yet")
	}

	var lastCommitHash string
	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(commitsDir, entry.Name())
			if hash, err := calculateCommitHash(dirPath); err == nil {
				lastCommitHash = hash
			}
		}
	}

	return lastCommitHash, nil
}

func calculateCommitHash(dirPath string) (string, error) {
	hasher := sha256.New()
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		hasher.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func getTrackedFiles() ([]string, error) {
	content, err := ioutil.ReadFile(indexFile)
	if err != nil {
		return nil, err
	}
	files := strings.Split(strings.TrimSpace(string(content)), "\n")
	return files, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func appendToFile(filePath, content string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
