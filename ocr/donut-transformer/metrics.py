from multiprocessing import Pool
from collections import defaultdict
import numpy as np
import nltk
from nltk import edit_distance

def compute_metrics(pred, gt, minlen=4):
    metrics = {}
    if len(pred) < minlen or len(gt) < minlen:
        return metrics
    metrics["edit_dist"] = edit_distance(pred, gt) / max(len(pred), len(gt))
    reference = gt.split()
    hypothesis = pred.split()
    metrics["bleu"] = nltk.translate.bleu([reference], hypothesis)
    try:
        metrics["meteor"] = nltk.translate.meteor([reference], hypothesis)
    except LookupError:
        metrics["meteor"] = np.nan
    reference = set(reference)
    hypothesis = set(hypothesis)
    metrics["precision"] = nltk.scores.precision(reference, hypothesis)
    metrics["recall"] = nltk.scores.recall(reference, hypothesis)
    metrics["f_measure"] = nltk.scores.f_measure(reference, hypothesis)
    return metrics

def get_metrics(gt: list[str], pred: list[str], pool: bool = True):
    metrics = defaultdict(list)
    if pool:
        with Pool() as p:
            _metrics = p.starmap(compute_metrics, iterable=zip(pred, gt))
    else:
        _metrics = [compute_metrics(p, g) for p, g in zip(pred, gt)]
    for m in _metrics:
        for key, value in m.items():
            metrics[key].append(value)
    return dict(metrics)