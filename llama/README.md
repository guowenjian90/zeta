
The quick steps to trigger LLama-Vicuna inference on GPU/CPU base server:

## Precondition:
### weights
LLama weights is in huge size and need to be ready before running inferenece, not provided in github directly so far.
Note: it's not allowed for commecial purposes at the moment(06032023). 

#### Llama weight:
Huggingface version:  https://huggingface.co/lmsys/vicuna-7b-delta-v1.1

##### 7B
```
from transformers import AutoTokenizer, AutoModelForCausalLM

#tokenizer = AutoTokenizer.from_pretrained("huggyllama/llama-7b")
#model = AutoModelForCausalLM.from_pretrained("huggyllama/llama-7b")

tokenizer = AutoTokenizer.from_pretrained("lmsys/vicuna-7b-delta-v1.1")
model = AutoModelForCausalLM.from_pretrained("lmsys/vicuna-7b-delta-v1.1")
```

##### 13B
```
from transformers import AutoTokenizer, AutoModelForCausalLM

tokenizer = AutoTokenizer.from_pretrained("huggyllama/llama-13b")
model = AutoModelForCausalLM.from_pretrained("huggyllama/llama-13b")
```


* suggest to enable conda environment or run it in docker containers. 

### Environment Variables
```
LLAMA_WEIGHT_7B_HF='/hf/weight/llama'
VICUNA_WEIGHT_7B_HF='./hf/weight/vicuna_7b'
VICUNA_7B_MODEL='/hf/model/vicuna_7b'
```

### hugginface.transformers
```
pip3 install transformers				

```
or
```
git clone https://github.com/huggingface/transformers.git
cd transformers
pip install -e .
```

### Convert Llama weight to huggingface
* find the transformers utility, take 7B weight as an example here:
```
python src/transformers/models/llama/convert_llama_weights_to_hf.py --input_dir /llama/weights/path --model_size 7B --output_dir $LLAMA_WEIGHT_7B_HF
```

#### notes
after convertsion, then model and tokenizer can be loaded through:
```
from transformers import LlamaForCausalLm, LlamTokenizer

tokenzier = LlamaTokenizer.from_pretrained("hf/llama/weight/path")
model = LlamaCausalLM.from_pretrained("hf/llama/weight/path")
```

## Llama-vicuna
### Vicuna installaition:
#### pip way
```
pip3 install fschat
```

#### from github
```
git clone https://github.com/lm-sys/FastChat.git
cd FastChat
pip3 install --upgrade pip #PEP 600 support
pip3 install -e .
```

### Vicuna weight
code references:
```
from transformers import AutoTokenizer, AutoModelForCausalLM

tokenizer = AutoTokenizer.from_pretrained("lmsys/vicuna-7b-delta-v1.1")

model = AutoModelForCausalLM.from_pretrained("lmsys/vicuna-7b-delta-v1.1")
```

### Run inference
```
cd FastChat # if installed from github
python3 -m fastchat.model.apply_dleta \
  --base-model-path $LLAMA_WEIGHT_7B_HF \
  --delta-path $VICUNA_WEIGHT_7B_HF     \
  --target-model-path $VICUNA_7B_MODEL
```




```
