from datasets import load_dataset

dataset = load_dataset("./output")
image = dataset['train']['image']
latex = dataset['train']['latex']
print(latex)