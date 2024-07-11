package src

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gen2brain/go-fitz"
)

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

func ProcessTexFile(DOC_HEAD string, filePath string, bashPath string, pngBasePath string) {
	fmt.Printf("Processing file: %s\n", filePath)

	latexContent, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", filePath, err)
		return
	}
	tables := extractTables(string(latexContent))

	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	parentDir := filepath.Base(filepath.Dir(filePath))
	baseName := parentDir + "_" + filename

	for i, table := range tables {
		fullLatex := createFullLatexDocument(DOC_HEAD, table)

		tmpName := fmt.Sprintf("%s_table_%d.", baseName, i)
		// tableTexFile := filepath.Join(filepath.Dir(filePath), fmt.Sprintf("%stex", tmpName))
		// tablePdfFile := filepath.Join(filepath.Dir(filePath), fmt.Sprintf("%spdf", tmpName))
		tableTexFile := filepath.Join(pngBasePath, fmt.Sprintf("%stex", tmpName))
		tablePdfFile := filepath.Join(pngBasePath, fmt.Sprintf("%spdf", tmpName))

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
			continue
		}

		err = convertPDFtoPNG(tablePdfFile, pngBasePath)
		if err != nil {
			fmt.Printf("Error converting PDF to PNG for %s: %v\n", filePath, err)
			// os.Remove(tableTexFile)
			// os.Remove(tablePdfFile)
			continue
		}
		// os.Remove(tableTexFile)
		// os.Remove(tablePdfFile)
		fmt.Printf("Table %d from %s converted to %s\n", i+1, filePath, pngBasePath)
	}
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
	return `\documentclass[10]{standalone}
` + docHead + `
\begin{document}
` + table + `
\end{document}`
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

func convertPDFtoPNG(pdfFile, outputDir string) error {
	doc, err := fitz.New(pdfFile)
	if err != nil {
		return fmt.Errorf("error opening PDF: %v", err)
	}
	defer doc.Close()

	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	for n := 0; n < doc.NumPage(); n++ {
		img, err := doc.Image(n)
		if err != nil {
			return fmt.Errorf("error rendering page %d: %v", n+1, err)
		}

		outFile := filepath.Join(outputDir, convertPNGName(pdfFile))
		f, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("error creating output file: %v", err)
		}

		err = png.Encode(f, img)
		if err != nil {
			f.Close()
			return fmt.Errorf("error encoding PNG: %v", err)
		}
		f.Close()

		fmt.Printf("Converted page %d to %s\n", n+1, outFile)
	}

	return nil
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
