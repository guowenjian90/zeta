# Zeta, Unleash the Power of Generative AI


## TODO List
### 1. End-2-End TODO
- [x] 1. gradio UI (front end) trigger diffusion modle inference call (backend)
- [X] 2. inpaint model demo example
### 2. Inferencece TODO
- [x] 1. install triton server and sevice on kubernetes through single yaml file
- [ ] 2. convert model checkpoint file to hugging face's diffussion model folder fomat, use local file replace "model id"
- [ ] 3. pack a docker image with model file, instead of downloading online
- [ ] 4. collect model serving performance metrics (native diffusion model call, no triton server), including, latency, throughput.
- [ ] 5. convert diffusion model to triton format, collect model serving perfoance metric with triton server, refer to [triton guide](https://github.com/triton-inference-server/tutorials/tree/main/Conceptual_Guide/Part_6-building_complex_pipelines)
- [ ] 6. triton server performance analysis and improvement design, or new design -- zeta serving
### 3. Fine-tune TODO
- [ ] 1. how to do multiple pods, elastic training
- [ ] 2. Pytorch Distributed https://pytorch.org/tutorials/beginner/dist_overview.html 
- [ ] 3. Open-source Distributed Training Library 1: https://huggingface.co/docs/accelerate/index
- [ ] 4. Open-source Distributed Training Library 2: https://github.com/hpcaitech/ColossalAI/tree/main
### 4. Cluster TODO
- [ ] 1. scheduler design consider both the quick serving request and long fine-tuning request (one cluster handle both services)
- [ ] 2. user model and data managment, fast storage service design, considering isolation and privacy
### 5. UI TODO
- [ ] 1. Seperate UI frontend from backend, docker image and service call design
- [ ] 2. gradio UI add one model option, and coresponding backend model definition (LLama)
- [ ] 3. gradio UI add fine-tune option, and coresponding backend model definition (fine-tune environment is more complex)

## Start the web server 
```docker run --rm --gpus all -p 0.0.0.0:80:7860 centaurusinfra/zeta_ui```

Access the UI with ```<YOUR Public IP>:80```


## Start the stable diffusion inpainting web server
```docker run --rm --gpus all -p 0.0.0.0:80:7860 centaurusinfra/zeta_ui_inpainting```

Access the UI with ```<YOUR Public IP>:80```

