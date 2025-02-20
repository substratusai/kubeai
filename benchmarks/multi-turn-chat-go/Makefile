data/raw/ShareGPT_V3_unfiltered_cleaned_split.json:
	mkdir -p data/raw
	cd data/raw && wget https://huggingface.co/datasets/anon8231489123/ShareGPT_Vicuna_unfiltered/resolve/main/ShareGPT_V3_unfiltered_cleaned_split.json

.PHONY: data
data: data-tiny data-small data-medium data-large data-xlarge

.PHONY: data-tiny
data-tiny: data/raw/ShareGPT_V3_unfiltered_cleaned_split.json
	cd data && python prepare-input-threads.py --output tiny.json --max-threads 6 --min-content-length 10 --min-message-count 2
	cat data/tiny.json | grep '"id":' | wc -l

.PHONY: data-small
data-small: data/raw/ShareGPT_V3_unfiltered_cleaned_split.json
	cd data && python prepare-input-threads.py --output small.json --max-threads 60 --min-content-length 100 --min-message-count 5
	cat data/small.json | grep '"id":' | wc -l

.PHONY: data-medium
data-medium: data/raw/ShareGPT_V3_unfiltered_cleaned_split.json
	cd data && python prepare-input-threads.py --output medium.json --max-threads 6000 --min-content-length 1000 --min-message-count 5
	cat data/medium.json | grep '"id":' | wc -l

.PHONY: data-large
data-large: data/raw/ShareGPT_V3_unfiltered_cleaned_split.json
	cd data && python prepare-input-threads.py --output large.json --max-threads 12000 --min-content-length 200 --min-message-count 5
	cat data/large.json | grep '"id":' | wc -l

.PHONY: data-xlarge
data-xlarge: data/raw/ShareGPT_V3_unfiltered_cleaned_split.json
	# NOTE: 32000 threads is close to the max that these filters amount to.
	cd data && python prepare-input-threads.py --output xlarge.json --max-threads 24000 --min-content-length 200 --min-message-count 3
	cat data/xlarge.json | grep '"id":' | wc -l

.PHONY: test
test:
	go test -v ./benchmark

IMG ?= us-central1-docker.pkg.dev/substratus-dev/default/benchmark-multi-turn-chat-go

.PHONY: build-docker-image
build-docker-image:
	docker build . -t ${IMG}

.PHONY: push-docker-image
push-docker-image:
	docker push ${IMG}