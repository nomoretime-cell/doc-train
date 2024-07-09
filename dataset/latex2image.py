import os
import subprocess
from pdf2image import convert_from_path
from PIL import Image

def latex_table_to_image(latex_table, output_image_path, timeout=30):
    temp_dir = "temp"
    os.makedirs(temp_dir, exist_ok=True)
    
    latex_file_path = os.path.join(temp_dir, "table.tex")
    with open(latex_file_path, "w") as f:
        f.write(r"""
        \documentclass{standalone}
        \usepackage{amsmath, amssymb}
        \usepackage{booktabs}
        \usepackage{xspace}
        \newcommand{\oursb}{{\textbf{\mbox{Donut}}}\xspace}  % 定义\oursb命令
        \begin{document}
        """)
        f.write(latex_table)
        f.write(r"""
        \end{document}
        """)
    
    try:
        result = subprocess.run(
            ["pdflatex", "-interaction=nonstopmode", "-output-directory", temp_dir, latex_file_path],
            check=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout
        )
        print(result.stdout.decode())
    except subprocess.TimeoutExpired:
        print(f"pdflatex command timed out after {timeout} seconds.")
        return
    except subprocess.CalledProcessError as e:
        print("Error in LaTeX processing:", e.stdout.decode(), e.stderr.decode())
        return
    
    pdf_path = os.path.join(temp_dir, "table.pdf")
    images = convert_from_path(pdf_path)
    
    if images:
        image = images[0]
        image.save(output_image_path)
    else:
        print("No images were generated from the PDF.")
    
    for file in os.listdir(temp_dir):
        os.remove(os.path.join(temp_dir, file))
    os.rmdir(temp_dir)

latex_table = r"""
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
\oursb \textbf{(Proposed)}&  & 143M & \textbf{752} & \textbf{95.30} \\
\bottomrule
\end{tabular}
"""

latex_table_2 = r"""
\begin{table}[h]
\begin{displaymath}
\begin{array}{lrr} \hline\hline\\
& \multicolumn{1}{c}{\rm thick~~slice}  &
\multicolumn{1}{c}{\rm thin~~slice}
\\[2mm] \hline \\
P_{2\rho}({\bf K}): &
|{\bf K}|^n;
&|{\bf K}|^{n+m/2} \\[2mm] 
\hline \\[3mm]
P_{2v}({\bf K}): & |{\bf K}|^{-3-m/2}; &
|{\bf K}|^{-3+m/2} \\[3mm] \hline
\end{array}
\end{displaymath}
\caption{Asymptotics of the  components of 2D spectrum in the {\it thin}
and {\it thick} velocity slices, $m=-\nu-3$.}
\label{tab:2Dspk_asymp}
\end{table}
"""

latex_table_to_image(latex_table, "output_image.png")