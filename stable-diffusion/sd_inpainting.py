import gradio as gr
import torch
from diffusers import StableDiffusionInpaintPipeline

# ---- API call to the model ----
def generate(dict, prompt):
    image =  dict['image'].convert("RGB").resize((512, 512))
    mask_image = dict['mask'].convert("RGB").resize((512, 512))

    # TODO change model id to local directory for better performance
    model_id = "runwayml/stable-diffusion-inpainting"
    pipe = StableDiffusionInpaintPipeline.from_pretrained(
        model_id,
        torch_dtype=torch.float16,
    )
    if torch.cuda.is_available():
        pipe.to("cuda")
    images = pipe(prompt=prompt, image=image, mask_image=mask_image).images
    return(images[0])

# ---- Web UI design and API hook up ----
gr.Interface(
    generate,
    title = 'Stable Diffusion In-Painting',
    inputs=[
        gr.Image(source = 'upload', tool = 'sketch', type = 'pil'),
        gr.Textbox(label = 'prompt')
    ],
    outputs = [
        gr.Image()
        ]
).launch(server_name="0.0.0.0",server_port=7860, debug=True, share=True)
