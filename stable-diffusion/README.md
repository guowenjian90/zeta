## Web UI
- Based on Automatic1111
- changes: seperate lanuch.py's prepare_env and start function, so library installed from prepare_env is built into docker image
## Model choices
- Stable diffusion base model + various lora style from civitai

## Notes
- when launch image with docker run, don't forget to enable gpu with option ```--gpus all```

  ```docker run -it --rm --gpus all <image_name> bash```
