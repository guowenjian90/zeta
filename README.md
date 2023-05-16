# Zeta, unleash GPT applications


## Start the web server 
```docker run --rm --gpus all -p 0.0.0.0:80:7860 centaurusinfra/zeta_ui```

Access the UI with ```<YOUR Public IP>:80```


## Start the stable diffusion inpainting web server
```docker run --rm --gpus all -p 0.0.0.0:80:7860 centaurusinfra/zeta_ui_inpainting```

Access the UI with ```<YOUR Public IP>:80```

## To-do list

- [x] 1. gradio UI trigger diffusion modle inference call
- [x] 2. install triton server and sevice on kubernetes through single yaml file
- [ ] 3. convert model checkpoint file to hugging face's diffussion model folder fomat, use local file replace "model id"
- [ ] 4. pack a docker image with model file, instead of downloading online
- [ ] 5. collect model serving performance metrics (native diffusion model call, no triton server), including, latency, throughput.
- [ ] 6. convert diffusion model to triton format, collect model serving perfoance metric with triton server
- [ ] 7. gradio UI add one model option, and coresponding backend model definition
- [ ] 8. triton server performance analysis and improvement design, or new design -- zeta serving
- [ ] 9. gradio UI add fine-tune option, and coresponding backend model definition
- [ ] 10. scheduler design consider both the quick serving request and long fine-tuning request 
- [ ] 11. user model and data managment, fast storage service design, considering isolation and privacy
