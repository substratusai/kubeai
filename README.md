# Lingo - LLM Serving

Inference server and gateway for Large Language Models

Insert demo gif

🚀 Serve popular OSS LLM models in minutes on CPUs or GPUs  
⚖️  Automatically scale your LLM up and down based on requests  
⚖️  Scale to 0 to save costs  
⬆️  Built-in gateway that batches requests while scaling happens  
☁️  Provide a unified API across clouds for serving LLMs  

Support the project by adding a star! ❤️

Join us on Discord:  
<a href="https://discord.gg/JeXhcmjZVm">
<img alt="discord-invite" src="https://dcbadge.vercel.app/api/server/JeXhcmjZVm?style=flat">
</a>

## Quickstart

Create a file named `mistral-7b.yaml` with following content:
```yaml
name: mistral-7b-vllm
serving: vllm
model:
  source:
    huggingface:
      id: hf-org/mistral-7b

# Remove if running CPU only
resources:
  accelerators:
    name: nvidia-l4
    count: 1

# replicas: 1 # just use min max
autoscaling:
  min: 0 # default is 0
  max: 3 # default is 3
  concurrentRequests: 4 # default is 16
```


CLI
```
lingo serve -f mistral-7b.yaml # OR
lingo serve mistral-7b --huggingface-model hf_org/model_id --min 0 --max 3
> Your endpoint is available at http://xx.com/api/mistral-7b
```

Run inference:
```
curl /api/mistral-7b
```

## Creators
Feel free to contact any of us:
* [Nick Stogner](https://www.linkedin.com/in/nstogner/)
* [Sam Stoelinga](https://www.linkedin.com/in/samstoelinga/)
