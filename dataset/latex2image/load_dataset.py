from datasets import load_dataset

dataset = load_dataset("./output")
image = dataset['train']['image']
latex = dataset['train']['ground_truth']
print(latex)