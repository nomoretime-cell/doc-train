package src

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gen2brain/go-fitz"
)

var MAP_DATASET_COUNT = make(map[string]int)

func FindTexFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".tex") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func ExtractPreamble(content string, basePath string) (string, error) {
	reStart := regexp.MustCompile(`(?m)^\s*\\documentclass.*$`)
	reEnd := regexp.MustCompile(`(?m)^\s*\\begin\{document\}.*$`)

	startIndex := reStart.FindStringIndex(content)
	if startIndex == nil {
		return "", fmt.Errorf("未找到 \\documentclass")
	}

	endIndex := reEnd.FindStringIndex(content[startIndex[1]:])
	if endIndex == nil {
		return "", fmt.Errorf("未找到 \\begin{document}")
	}

	fullPreamble := content[startIndex[1] : startIndex[1]+endIndex[0]]

	// 只保留 \input 和 \newcommand 开头的行
	reKeep := regexp.MustCompile(`(?m)^\s*(\\input|\\newcommand).*$`)
	matches := reKeep.FindAllString(fullPreamble, -1)

	preamble := strings.Join(matches, "\n")

	// 处理前导区中的 \input 命令
	processedPreamble, err := processInput(preamble, basePath)
	if err != nil {
		return "", err
	}

	// 再次筛选，只保留 \newcommand 和 \def 开头的行
	reFinalKeep := regexp.MustCompile(`(?m)^\s*(\\newcommand|\\def).*$`)
	finalMatches := reFinalKeep.FindAllString(processedPreamble, -1)

	finalPreamble := strings.Join(finalMatches, "\n")

	return strings.TrimSpace(finalPreamble), nil
}

func ProcessTexFile(DOC_HEAD string, filePath string, bashPath string, trainDataset string, totalDatasetCount int, isDebug bool) bool {
	if _, exists := MAP_DATASET_COUNT[trainDataset]; !exists {
		MAP_DATASET_COUNT[trainDataset] = 0
	}

	if _, err := os.Stat(trainDataset); os.IsNotExist(err) {
		err := os.MkdirAll(trainDataset, 0755)
		if err != nil {
			fmt.Errorf("failed to create directory: %v", err)
			return false
		}
	}

	fmt.Printf("Processing file: %s\n", filePath)

	latexContent, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", filePath, err)
		return true
	}
	tables := extractTables(string(latexContent))

	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	parentDir := filepath.Base(filepath.Dir(filePath))
	baseName := parentDir + "_" + filename

	for i, table := range tables {
		if MAP_DATASET_COUNT[trainDataset] >= totalDatasetCount {
			return false
		}

		// remove tabs and spaces
		table = strings.ReplaceAll(table, "\t", "")
		table = strings.ReplaceAll(table, " ", "")
		fullLatex := createFullLatexDocument(DOC_HEAD, table)

		tmpName := fmt.Sprintf("%s_table_%d.", baseName, i)
		// tableTexFile := filepath.Join(filepath.Dir(filePath), fmt.Sprintf("%stex", tmpName))
		// tablePdfFile := filepath.Join(filepath.Dir(filePath), fmt.Sprintf("%spdf", tmpName))
		tableTexFile := filepath.Join(trainDataset, fmt.Sprintf("%stex", tmpName))
		tablePdfFile := filepath.Join(trainDataset, fmt.Sprintf("%spdf", tmpName))
		tmpLog := filepath.Join(trainDataset, fmt.Sprintf("%slog", tmpName))
		tmpAux := filepath.Join(trainDataset, fmt.Sprintf("%saux", tmpName))

		err = os.WriteFile(tableTexFile, []byte(fullLatex), 0644)
		if err != nil {
			fmt.Printf("Error writing temp file for %s: %v\n", filePath, err)
			continue
		}

		err = compileLaTeX(tableTexFile, tablePdfFile)
		if err != nil {
			fmt.Printf("Error compiling LaTeX for %s: %v\n", filePath, err)
		}
		if !FolderExists(tablePdfFile) {
			os.Remove(tableTexFile)
			os.Remove(tmpLog)
			os.Remove(tmpAux)
			continue
		}

		pngFileName, err := convertPDFtoPNG(tablePdfFile, trainDataset)
		if !isDebug {
			os.Remove(tableTexFile)
			os.Remove(tablePdfFile)
			os.Remove(tmpLog)
			os.Remove(tmpAux)
		}

		if err != nil {
			fmt.Printf("Error converting PDF to PNG for %s: %v\n", filePath, err)
			continue
		}
		fmt.Printf("Table %d from %s converted to %s\n", i+1, filePath, trainDataset)

		// remove \n and \r , save "latex table" and "png" into metainfo.jsonl
		table = strings.ReplaceAll(table, "\n", "")
		table = strings.ReplaceAll(table, "\r", "")
		newMetadata := Metadata{
			FileName: pngFileName,
			Latex:    table,
		}
		appendMetaInfo(newMetadata, trainDataset)

		MAP_DATASET_COUNT[trainDataset]++
	}
	return true
}

func appendMetaInfo(newMetadata Metadata, bashpath string) {
	// 打开文件，使用追加模式
	file, err := os.OpenFile(filepath.Join(bashpath, "metadata.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	// 将结构体转换为 JSON
	jsonData, err := json.Marshal(newMetadata)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}

	// 写入 JSON 行到文件
	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}

	// 写入换行符
	_, err = file.WriteString("\n")
	if err != nil {
		fmt.Println("Error writing newline to file:", err)
		return
	}

	fmt.Println("New data successfully appended to metadata.jsonl")
}

func processInput(content string, basePath string) (string, error) {
	inputRegex := regexp.MustCompile(`\\input\{([^}]+)\}`)

	processedContent := inputRegex.ReplaceAllStringFunc(content, func(match string) string {
		filename := inputRegex.FindStringSubmatch(match)[1]
		fullPath := filepath.Join(basePath, filename)

		// 如果文件名没有扩展名，添加 .tex 扩展名
		if filepath.Ext(fullPath) == "" {
			fullPath += ".tex"
		}

		fileContent, err := loadFile(fullPath)
		if err != nil {
			fmt.Printf("警告：无法加载文件 %s: %v\n", fullPath, err)
			return match // 如果无法加载文件，保留原始的 \input 命令
		}

		// 递归处理加载的文件中的 \input 命令
		processedFileContent, _ := processInput(fileContent, filepath.Dir(fullPath))
		return processedFileContent
	})

	return processedContent, nil
}

func extractTables(content string) []string {
	// 第一步：提取 \begin{table} 和 \end{table} 之间的内容
	tableRe := regexp.MustCompile(`(?s)\\begin{table}(.*?)\\end{table}`)
	tables := tableRe.FindAllStringSubmatch(content, -1)

	// 第二步：从每个table中提取 \begin{tabular} 部分
	tabularRe := regexp.MustCompile(`(?s)\\begin{tabular}.*?\\end{tabular}`)
	var result []string

	for _, table := range tables {
		if len(table) > 1 {
			tabular := tabularRe.FindString(table[1])
			if tabular != "" {
				result = append(result, tabular)
			}
		}
	}

	return result
}

func createFullLatexDocument(docHead string, table string) string {
	originalLatex := `\documentclass{standalone}
` + docHead + `
\begin{document}
` + table + `
\end{document}`

	return originalLatex
}

func compileLaTeX(inputFile, outputFile string) error {
	cmd := exec.Command("pdflatex", "-interaction=nonstopmode", "-output-directory="+filepath.Dir(outputFile), inputFile)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("LaTeX compilation failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func convertPDFtoPNG(pdfFile, outputDir string) (string, error) {
	doc, err := fitz.New(pdfFile)
	if err != nil {
		return "", fmt.Errorf("error opening PDF: %v", err)
	}
	defer doc.Close()

	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return "", fmt.Errorf("error creating output directory: %v", err)
	}

	pngFileName := convertPNGName(pdfFile)
	if doc.NumPage() > 1 {
		return "", fmt.Errorf("PDF contains multiple pages")
	}

	n := 0
	img, err := doc.Image(n)
	if err != nil {
		return "", fmt.Errorf("error rendering page %d: %v", n+1, err)
	}

	outFile := filepath.Join(outputDir, pngFileName)
	f, err := os.Create(outFile)
	if err != nil {
		return "", fmt.Errorf("error creating output file: %v", err)
	}

	err = png.Encode(f, img)
	if err != nil {
		f.Close()
		return "", fmt.Errorf("error encoding PNG: %v", err)
	}
	f.Close()

	fmt.Printf("Converted page %d to %s\n", n+1, outFile)

	return pngFileName, nil
}

func convertPNGName(input string) string {
	parts := strings.Split(input, string(filepath.Separator))

	if len(parts) < 2 {
		return input // 如果路径部分少于2，返回原始输入
	}
	pdfFileName := parts[len(parts)-1]
	result := strings.TrimSuffix(pdfFileName, filepath.Ext(pdfFileName)) + ".png"

	return result
}
