from transformers import AutoModelForTokenClassification
from transformers import AutoProcessor
from datasets import load_dataset
from PIL import ImageDraw, ImageFont
import torch


def unnormalize_box(bbox, width, height):
    return [
        width * (bbox[0] / 1000),
        height * (bbox[1] / 1000),
        width * (bbox[2] / 1000),
        height * (bbox[3] / 1000),
    ]


model = AutoModelForTokenClassification.from_pretrained("test/checkpoint-1000")
dataset = load_dataset("funsd-layoutlmv3")
example = dataset["test"][0]
image = example["image"]
words = example["tokens"]
boxes = example["bboxes"]
word_labels = example["ner_tags"]

processor = AutoProcessor.from_pretrained("layoutlmv3-base", apply_ocr=False)

encoding = processor(
    image, words, boxes=boxes, word_labels=word_labels, return_tensors="pt"
)
for k, v in encoding.items():
    print(k, v.shape)


with torch.no_grad():
    outputs = model(**encoding)

logits = outputs.logits
predictions = logits.argmax(-1).squeeze().tolist()
labels = encoding.labels.squeeze().tolist()

token_boxes = encoding.bbox.squeeze().tolist()
width, height = image.size

true_predictions = [
    model.config.id2label[pred]
    for pred, label in zip(predictions, labels)
    if label != -100
]
true_labels = [
    model.config.id2label[label]
    for prediction, label in zip(predictions, labels)
    if label != -100
]
true_boxes = [
    unnormalize_box(box, width, height)
    for box, label in zip(token_boxes, labels)
    if label != -100
]

draw = ImageDraw.Draw(image)

font = ImageFont.load_default()


def iob_to_label(label):
    label = label
    if not label:
        return "other"
    return label


label2color = {
    "b-question": "blue",
    "i-question": "blue",
    "o": "black",
    "b-answer": "green",
    "i-answer": "green",
    "b-header": "orange",
    "i-header": "orange",
    "other": "violet",
}

for prediction, box in zip(true_predictions, true_boxes):
    predicted_label = iob_to_label(prediction).lower()
    draw.rectangle(box, outline=label2color[predicted_label])
    draw.text(
        (box[0] + 10, box[1] - 10),
        text=predicted_label,
        fill=label2color[predicted_label],
        font=font,
    )

image.save("saved_image.jpg")
