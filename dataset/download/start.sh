#!/bin/bash

nohup python download.py --manifest_file ./arXiv_src_manifest.xml  --mode src --output_dir output > log 2>&1 &
