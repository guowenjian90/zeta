import gradio as gr
import torch
from diffusers import StableDiffusionPipeline

# ---- API call to the model ----
def generate(prompt):
    model_id = "runwayml/stable-diffusion-v1-5"
    pipeline = StableDiffusionPipeline.from_pretrained(model_id,torch_dtype=torch.float32)
    if torch.cuda.is_available():
        pipeline.to("cuda")
    image = pipeline(prompt).images[0]
    return image

# ---- Web UI design and API hook up ----
with gr.Blocks() as app:
    prompt = gr.Textbox(label="Prompt")
    submit_btn = gr.Button("Generate")
    img_output = gr.Image().style(height=512)
    submit_btn.click(fn=generate, inputs=prompt, outputs=img_output)

# ---- Start the Web server ---- 
app.launch(debug=True, share=True)
