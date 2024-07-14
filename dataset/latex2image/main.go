package main

import (
	"fmt"
	"latex2image/src"
	"os"
	"path/filepath"
	"strings"
)

var TRAIN_DATASET string = "output/train"
var TRAIN_COUNT int = 8
var TEST_DATASET string = "output/test"
var TEST_COUNT int = 2
var VALIDATION_DATASET string = "output/validation"
var VALIDATION_COUNT int = 2
var IS_DEBUG bool = false

var isTrainContinue bool = true
var isTestContinue bool = true
var isValidationContinue bool = true

func main() {
	rootDir := "./arXiv"
	isContinue := readArXivTar(rootDir)
	if !isContinue {
		fmt.Println("read arXiv finished")
	}
}

func readArXivTar(path string) bool {
	yearIndexTars, err := os.ReadDir(path)
	if err != nil {
		fmt.Println("Error reading directory:", err)
	}

	for _, yearIndexTar := range yearIndexTars {
		// yearIndexTar = arXiv_src_2003_026.tar
		if yearIndexTar.IsDir() || !strings.HasSuffix(yearIndexTar.Name(), ".tar") {
			continue
		}

		// yearIndex = 2003_026
		yearIndex := src.GetArXivYearIndex(yearIndexTar.Name())
		if yearIndex == "" {
			fmt.Println("Error extracting year from filename:", yearIndexTar.Name())
			return true
		}

		yearIndexTarPath := filepath.Join(path, yearIndexTar.Name())
		yearIndexFolder := filepath.Join(path, yearIndex)

		if !src.FolderExists(yearIndexFolder) {
			fmt.Println("file folder not exists and need to decompression")
			err := src.ExtractTar(yearIndexTarPath, yearIndexFolder)
			if err != nil {
				fmt.Printf("decompression error: %v\n", err)
			} else {
				fmt.Println("decompression success")
			}
		} else {
			fmt.Printf("folder %s is exists, do not need to be created\n", yearIndex)
		}

		// read paper in yearIndexFolder
		isContinue := readPaperGz(yearIndexFolder)
		if !isContinue {
			return false
		}
	}
	return true
}

func readPaperGz(basePath string) bool {
	paperGzs, err := os.ReadDir(basePath)
	if err != nil {
		fmt.Println("Error reading directory:", err)
	}
	for _, paperGz := range paperGzs {
		if paperGz.IsDir() || !strings.HasSuffix(paperGz.Name(), ".gz") {
			continue
		}

		paperGzPath := filepath.Join(basePath, paperGz.Name())
		paperFolder := filepath.Join(basePath, strings.TrimSuffix(paperGz.Name(), ".gz"))

		if !src.FolderExists(paperFolder) {
			fmt.Println("file folder not exists and need to be created")
			err := src.ExtractTar(paperGzPath, paperFolder)
			if err != nil {
				fmt.Printf("unzip error: %v\n", err)
			} else {
				fmt.Println("unzip success")
			}
		} else {
			fmt.Printf("folder %s is exists, do not need to be created\n", paperFolder)
		}

		// find all .tex files from paper folder
		paperTexFiles, err := src.FindTexFiles(paperFolder)
		if err != nil {
			fmt.Println("Error finding .tex files:", err)
			continue
		}

		// get predefined line from main tex file
		var docHead string = ""
		for _, paperTex := range paperTexFiles {
			latexContent, _ := os.ReadFile(paperTex)
			tmpDocHead, err := src.ExtractPreamble(string(latexContent), paperFolder)
			if err != nil {
				continue
			}
			docHead = tmpDocHead
			break
		}

		// generate png
		for _, paperTex := range paperTexFiles {
			if isTrainContinue {
				isTrainContinue = src.ProcessTexFile(docHead, paperTex, paperFolder, TRAIN_DATASET, TRAIN_COUNT, IS_DEBUG)
			} else if isTestContinue {
				isTestContinue = src.ProcessTexFile(docHead, paperTex, paperFolder, TEST_DATASET, TEST_COUNT, IS_DEBUG)
			} else if isValidationContinue {
				isValidationContinue = src.ProcessTexFile(docHead, paperTex, paperFolder, VALIDATION_DATASET, VALIDATION_COUNT, IS_DEBUG)
			}
			if !isTrainContinue && !isTestContinue && !isValidationContinue {
				return false
			}
		}
	}
	return true
}