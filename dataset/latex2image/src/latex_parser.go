package src

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

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

func ExtractPreamble(content string, basePath string) (result string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("occur panic: %v", r)
			result = ""
		}
	}()

	// reStart := regexp.MustCompile(`(?m)^\s*\\documentclass.*$`)
	// startIndex := reStart.FindStringIndex(content)
	// if startIndex == nil {
	// 	return "", fmt.Errorf("未找到 \\documentclass")
	// }

	reEnd := regexp.MustCompile(`(?m)^\s*\\begin\{document\}.*$`)
	endIndex := reEnd.FindStringIndex(content)
	if endIndex == nil {
		return "", fmt.Errorf("not found \\begin{document}")
	}

	fullPreamble := content[:endIndex[0]]

	// only keep \input \newcommand \def
	reKeep := regexp.MustCompile(`(?m)^\s*(\\input|\\newcommand|\\def).*$`)
	matches := reKeep.FindAllString(fullPreamble, -1)

	preamble := strings.Join(matches, "\n")

	// process \input
	processedPreamble, err := processInput(preamble, basePath, 0)
	if err != nil {
		return "", err
	}

	reFinalKeep := regexp.MustCompile(`(?m)^\s*(\\newcommand|\\def).*$`)
	finalMatches := reFinalKeep.FindAllString(processedPreamble, -1)

	finalPreamble := strings.Join(finalMatches, "\n")

	return finalPreamble, nil
}

func removeAfterPercent(s string) string {
	index := strings.Index(s, "%")
	if index != -1 {
		return s[:index]
	}
	return s
}

func containCommand(text string, commandMap map[string]string) bool {
	for name := range commandMap {
		contain := strings.Contains(text, name)
		if contain {
			return true
		}
	}
	return false
}

func replaceMacro(text string, macroMap map[string]string) string {
	for macroName, macroDef := range macroMap {
		text = strings.ReplaceAll(text, macroName, macroDef)
	}
	return text
}

func parseLaTeXMacro(def string) (string, string, string, string) {
	def = strings.TrimSpace(def)

	var macroName, macroDef string
	var commandName, commandDef string

	if strings.HasPrefix(def, "\\def") {
		def = strings.TrimPrefix(def, "\\def")
		def = strings.TrimLeftFunc(def, unicode.IsSpace) // remove \def space

		// get def name
		endOfMacroName := strings.IndexFunc(def, func(r rune) bool {
			return unicode.IsSpace(r) || r == '{'
		})
		if endOfMacroName == -1 {
			return "", "", "", ""
		}
		macroName = def[:endOfMacroName]
		def = def[endOfMacroName:]

		// get def value
		def = strings.TrimLeftFunc(def, unicode.IsSpace) // remove def space
		if !strings.HasPrefix(def, "{") {
			return "", "", "", ""
		}
		macroDef = strings.TrimPrefix(def, "{")
	} else if strings.HasPrefix(def, "\\newcommand") {
		// process \newcommand
		def = strings.TrimPrefix(def, "\\newcommand")
		def = strings.TrimLeftFunc(def, unicode.IsSpace) // remove \newcommand space

		// get name
		if !strings.HasPrefix(def, "{") {
			return "", "", "", ""
		}
		closeBrace := strings.Index(def, "}")
		if closeBrace == -1 {
			return "", "", "", ""
		}
		commandName = strings.Trim(def[:closeBrace+1], "{}")
		def = def[closeBrace+1:]

		// process param
		if strings.HasPrefix(def, "[") {
			closeBracket := strings.Index(def, "]")
			if closeBracket != -1 {
				def = def[closeBracket+1:]
			}
		}

		// process '{'
		if !strings.HasPrefix(def, "{") {
			return "", "", "", ""
		}

		commandDef = strings.TrimPrefix(def, "{")
	} else {
		return "", "", "", ""
	}

	macroDef = strings.TrimSuffix(macroDef, "}")
	commandDef = strings.TrimSuffix(commandDef, "}")

	return macroName, macroDef, commandName, commandDef
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
		// table = strings.ReplaceAll(table, " ", "")
		fullLatex := createFullLatexDocument(DOC_HEAD, table)
		if fullLatex == "" {
			continue
		}
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

		table = replace_norm(table)
		newMetadata := Metadata{
			FileName:    pngFileName,
			GroundTruth: table,
		}
		appendMetaInfo(newMetadata, trainDataset)

		MAP_DATASET_COUNT[trainDataset]++
	}
	return true
}

func replace_norm(input string) string {
	input = strings.ReplaceAll(input, "\\cr", "\\\\")
	input = strings.ReplaceAll(input, "\r", "")
	// input = strings.ReplaceAll(input, "\n", "[NEWLINE]")
	return input
}

func replace_latex(input string) string {
	var result strings.Builder
	input = strings.ReplaceAll(input, "\\begin{tabular}", "<s_table>")
	input = strings.ReplaceAll(input, "\\end{tabular}", "</s_table>")

	input = strings.ReplaceAll(input, "\\cr", "\\\\")
	input = strings.ReplaceAll(input, "\r\n", "\n")
	input = strings.ReplaceAll(input, "\n\r", "\n")
	rows := strings.Split(input, "\n")
	for _, row := range rows {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}

		isBegin := strings.Contains(row, "s_table")
		isRowEnd := strings.HasSuffix(row, "\\\\")
		if isBegin {
			row = strings.ReplaceAll(row, "{", "<s_column_type>")
			row = strings.ReplaceAll(row, "}", "</s_column_type>")
			result.WriteString(row)
		} else if !isRowEnd {
			result.WriteString("<s_attribute>")
			result.WriteString(row)
		} else {
			result.WriteString("<s_row>")

			columns := strings.Split(row, "&")
			for _, col := range columns {
				col = strings.TrimSpace(col)
				result.WriteString("<s_column>")
				result.WriteString(col)
				result.WriteString("</s_column>")
			}
		}

		if isBegin {
			// do nothing
		} else if !isRowEnd {
			result.WriteString("</s_attribute>")
		} else {
			result.WriteString("</s_row>")
		}
	}
	strResult := result.String()
	strResult = strings.ReplaceAll(strResult, "\\\\", "")
	return strResult
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

const maxRecursionDepth = 3

func processInput(content string, basePath string, depth int) (result string, err error) {
	if depth > maxRecursionDepth {
		return content, fmt.Errorf("达到最大递归深度 %d", maxRecursionDepth)
	}

	inputRegex := regexp.MustCompile(`\\input\{([^}]+)\}`)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("occur panic: %v", r)
			result = ""
		}
	}()
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
		processedFileContent, _ := processInput(fileContent, filepath.Dir(fullPath), depth+1)
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
	macroStrings := strings.Split(strings.TrimSpace(docHead), "\n")
	docHead = ""
	macroMap := make(map[string]string)
	commandMap := make(map[string]string)
	for _, macroString := range macroStrings {
		macroString = removeAfterPercent(macroString)
		macroName, macroDef, commandName, commandDef := parseLaTeXMacro(macroString)
		if macroName != "" && macroDef != "" {
			macroMap[macroName] = macroDef
			fmt.Printf("Macro added to map: %s -> %s\n", macroName, macroDef)
		}
		if commandName != "" && commandDef != "" {
			commandMap[commandName] = commandDef
			fmt.Printf("Command added to map: %s -> %s\n", commandName, commandDef)
		}
	}

	if containCommand(table, commandMap) {
		return ""
	}
	table = replaceMacro(table, macroMap)

	realTable := ""
	tmpTable := strings.Split(table, "\n")
	for _, tableLine := range tmpTable {
		tableLine = removeAfterPercent(tableLine)
		if tableLine == "" {
			continue
		}
		if realTable == "" {
			realTable = tableLine
		} else {
			realTable = realTable + "\n" + tableLine
		}
	}

	originalLatex := `\documentclass{standalone}
` + docHead + `
\begin{document}
` + realTable + `
\end{document}`

	return originalLatex
}

func compileLaTeX(inputFile, outputFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pdflatex", "-interaction=nonstopmode", "-output-directory="+filepath.Dir(outputFile), inputFile)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("LaTeX compilation timed out after 2 minutes")
		}
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

	// check image size
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// set min image size
	minWidth, minHeight := 100, 100
	if width < minWidth || height < minHeight {
		return "", fmt.Errorf("image size too small: %dx%d (minimum required: %dx%d)", width, height, minWidth, minHeight)
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
