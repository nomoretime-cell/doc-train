# !pip install -q donut-python

from networkx import predecessor
from metrics import get_metrics
import re
import json
import torch
from tqdm.auto import tqdm
import numpy as np
from donut import JSONParseEvaluator
from datasets import load_dataset


"""Note that you can also easily refer to a specific commit in the `from_pretrained` method using the [`revision`](https://huggingface.co/docs/transformers/v4.21.1/en/main_classes/model#transformers.PreTrainedModel.from_pretrained.revision) argument, or use the private hub in case you'd like to keep your models private and only shared with certain colleagues for instance.

Here we're just loading from the main branch, which means the latest commit.
"""

custom_dataset = "/home/yejibing/code/doc-parser/doc-train/dataset/latex2image/output"
base_model = "/home/yejibing/code/doc-parser/doc-train/ocr/donut-transformer/result/model_final"
torch.cuda.set_device(0)


from transformers import DonutProcessor, VisionEncoderDecoderModel

processor = DonutProcessor.from_pretrained(base_model)
model = VisionEncoderDecoderModel.from_pretrained(base_model)

"""As we don't have a test split here, let's evaluate on the validation split.

We'll use the `token2json` method of the processor to turn the generated sequences into JSON, and the `JSONParseEvaluator` object available in the Donut package.
"""

device = "cuda" if torch.cuda.is_available() else "cpu"

model.eval()
model.to(device)

gt = []
pr = []

dataset = load_dataset(custom_dataset, split="test")

for idx, sample in tqdm(enumerate(dataset), total=len(dataset)):
    # prepare encoder inputs
    pixel_values = processor(sample["image"].convert("RGB"), return_tensors="pt").pixel_values
    pixel_values = pixel_values.to(device)
    # prepare decoder inputs
    task_prompt = "<s_cord-v2>"
    decoder_input_ids = processor.tokenizer(task_prompt, add_special_tokens=False, return_tensors="pt").input_ids
    decoder_input_ids = decoder_input_ids.to(device)

    # autoregressively generate sequence
    outputs = model.generate(
            pixel_values,
            decoder_input_ids=decoder_input_ids,
            max_length=model.decoder.config.max_position_embeddings,
            early_stopping=True,
            pad_token_id=processor.tokenizer.pad_token_id,
            eos_token_id=processor.tokenizer.eos_token_id,
            use_cache=True,
            num_beams=1,
            bad_words_ids=[[processor.tokenizer.unk_token_id]],
            return_dict_in_generate=True,
        )

    # turn into JSON
    seq = processor.batch_decode(outputs.sequences)[0]
    seq = seq.replace(processor.tokenizer.eos_token, "").replace(processor.tokenizer.pad_token, "")
    seq = re.sub(r"<.*?>", "", seq, count=1).strip()  # remove first task start token
    seq = processor.token2json(seq)

    ground_truth = sample["ground_truth"]
    prediction = seq["text_sequence"]
    print(f"gt: {ground_truth}")
    print(f"prediction: {prediction}")
    
    gt.append(ground_truth)
    pr.append(prediction)
    
metrics = get_metrics(gt, pr, False)
print({key: sum(values) / len(values) for key, values in metrics.items()})


