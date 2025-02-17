data/ShareGPT_V3_unfiltered_cleaned_split.json:
	cd data && wget https://huggingface.co/datasets/anon8231489123/ShareGPT_Vicuna_unfiltered/resolve/main/ShareGPT_V3_unfiltered_cleaned_split.json

.PHONY: data
data: data/ShareGPT_V3_unfiltered_cleaned_split.json
	cd data && python prepare-input-threads.py

run: 
	ls scenarios/${SCENARIO}
	CONFIG_DIR=scenarios/${SCENARIO} DATA_DIR=data MODEL_ADDR=kubeai/openai k6 run ./k6.js