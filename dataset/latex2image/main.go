package main

import (
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gen2brain/go-fitz"
)

var globalPreamble string

func main() {
	rootDir := "." // 当前目录，你可以修改为其他路径

	// 首先找到主LaTeX文件并提取全局前导
	err := findMainTexAndExtractPreamble(rootDir)
	if err != nil {
		fmt.Println("Error finding main LaTeX file:", err)
		return
	}

	// 查找所有 .tex 文件
	texFiles, err := findTexFiles(rootDir)
	if err != nil {
		fmt.Println("Error finding .tex files:", err)
		return
	}

	// 处理每个 .tex 文件
	for _, texFile := range texFiles {
		processTexFile(texFile)
	}
}

func findMainTexAndExtractPreamble(root string) error {
	var mainFile string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".tex") {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(content), "\\documentclass") {
				mainFile = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if mainFile == "" {
		return fmt.Errorf("no main LaTeX file found")
	}

	content, err := ioutil.ReadFile(mainFile)
	if err != nil {
		return err
	}
	globalPreamble = extractPreamble(string(content))
	return nil
}

func findTexFiles(root string) ([]string, error) {
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

func processTexFile(filePath string) {
	fmt.Printf("Processing file: %s\n", filePath)

	// 读取LaTeX文件
	latexContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", filePath, err)
		return
	}

	// 检查文件是否有自己的documentclass
	hasOwnDocumentClass := strings.Contains(string(latexContent), "\\documentclass")

	// 决定使用哪个前导
	var preambleToUse string
	if hasOwnDocumentClass {
		preambleToUse = extractPreamble(string(latexContent))
	} else {
		preambleToUse = globalPreamble
	}

	// 提取表格
	tables := extractTables(string(latexContent))

	// 获取文件名（不包括扩展名）用于生成输出文件名
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	for i, table := range tables {
		// 创建完整的LaTeX文档
		fullLatex := createFullLatexDocument(preambleToUse, table)

		// 生成临时文件名
		tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s_table_%d.tex", baseName, i))
		err = ioutil.WriteFile(tempFile, []byte(fullLatex), 0644)
		if err != nil {
			fmt.Printf("Error writing temp file for %s: %v\n", filePath, err)
			continue
		}

		// 生成输出PDF文件名
		pdfFile := filepath.Join(filepath.Dir(filePath), fmt.Sprintf("%s_table_%d.pdf", baseName, i))

		// 编译LaTeX为PDF
		err = compileLaTeX(tempFile, pdfFile)
		if err != nil {
			fmt.Printf("Error compiling LaTeX for %s: %v\n", filePath, err)
			os.Remove(tempFile)
			continue
		}

		// 生成输出PNG文件名
		pngFile := filepath.Join(filepath.Dir(filePath), fmt.Sprintf("%s_table_%d.png", baseName, i))

		// 转换PDF为PNG
		err = convertPDFtoPNG(pdfFile, pngFile)
		if err != nil {
			fmt.Printf("Error converting PDF to PNG for %s: %v\n", filePath, err)
			os.Remove(tempFile)
			os.Remove(pdfFile)
			continue
		}

		// 清理临时文件
		os.Remove(tempFile)
		os.Remove(pdfFile)

		fmt.Printf("Table %d from %s converted to %s\n", i+1, filePath, pngFile)
	}
}

func extractPreamble(content string) string {
	re := regexp.MustCompile(`(?s)\\documentclass.*?\\begin{document}`)
	match := re.FindString(content)
	if match == "" {
		return ""
	}
	return strings.TrimSuffix(match, `\begin{document}`)
}

func extractTables(content string) []string {
	re := regexp.MustCompile(`(?s)\\begin{table}.*?\\end{table}`)
	return re.FindAllString(content, -1)
}

func createFullLatexDocument(preamble, table string) string {
	// 如果前导中没有documentclass，添加一个默认的
	if !strings.Contains(preamble, "\\documentclass") {
		preamble = "\\documentclass{article}\n" + preamble
	}

	// 确保必要的包被引入
	necessaryPackages := `
\usepackage{graphicx}
\usepackage{float}
\usepackage{amsmath}
\usepackage{booktabs}
\usepackage{array}
`
	if !strings.Contains(preamble, "\\usepackage{graphicx}") {
		preamble += necessaryPackages
	}

	return `
\documentclass{standalone}
\usepackage{amsmath, amssymb}
\usepackage{booktabs}
\usepackage{xspace}
\begin{document}
\begin{tabular}{lcccc}
\toprule
& OCR  & \#Params & Time (ms) & Accuracy (\%)\\
\midrule
BERT &\checkmark & 110M + $\alpha^{\dag}$ & 1392 & 89.81 \\ %
RoBERTa &\checkmark & 125M + $\alpha^{\dag}$  & 1392 & 90.06 \\ %
LayoutLM &\checkmark & 113M + $\alpha^{\dag}$  & 1396 & 91.78 \\
LayoutLM (w/ image) &\checkmark & 160M + $\alpha^{\dag}$  & 1426 & 94.42 \\
LayoutLMv2 &\checkmark & 200M + $\alpha^{\dag}$  & 1489 & {95.25} \\ %
\midrule
{{\textbf{\mbox{Donut}}}\xspace} \textbf{(Proposed)}&  & 143M & \textbf{752} & \textbf{95.30} \\
\bottomrule
\end{tabular}
\end{document}`
}

func compileLaTeX(inputFile, outputFile string) error {
	cmd := exec.Command("pdflatex", "-interaction=nonstopmode", "-output-directory="+filepath.Dir(outputFile), inputFile)
	return cmd.Run()
}

func convertPDFtoPNG(pdfFile, outputDir string) error {
	// 打开 PDF 文件
	doc, err := fitz.New(pdfFile)
	if err != nil {
		return fmt.Errorf("error opening PDF: %v", err)
	}
	defer doc.Close()

	// 确保输出目录存在
	err = os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	// 遍历 PDF 的每一页
	for n := 0; n < doc.NumPage(); n++ {
		// 将页面渲染为图像
		img, err := doc.Image(n)
		if err != nil {
			return fmt.Errorf("error rendering page %d: %v", n+1, err)
		}

		// 创建输出文件
		outFile := filepath.Join(outputDir, fmt.Sprintf("page_%d.png", n+1))
		f, err := os.Create(outFile)
		if err != nil {
			return fmt.Errorf("error creating output file: %v", err)
		}

		// 将图像编码为 PNG 并保存
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
