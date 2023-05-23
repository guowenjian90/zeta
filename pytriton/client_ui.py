


def generate(prompt):


    # prompts = [
    #     "A photo of an astronaut riding a horse on mars",
    #     "An image of a squirrel in Picasso style",
    #     "A running dog in the fields of trees in Manga style",
    # ]

    img_size = np.array([[512]])
    # results_path = pathlib.Path(args.results_path)
    # results_path.mkdir(parents=True, exist_ok=True)

    with ModelClient(args.url, "StableDiffusion_1_5", init_timeout_s=args.init_timeout_s) as client:
        logger.debug(f"Sending request.")
        prompt = np.array([[prompt]])
        prompt = np.char.encode(prompt, "utf-8")
        logger.info(f"Prompt: {prompt}")
        result_dict = client.infer_batch(prompt=prompt, img_size=img_size)
        logger.debug(f"Result for for request.")
        resp_img = result_dict['image'][0]
        #print(type(resp_img))
        #print(resp_img.shape)
        msg = base64.b64decode(resp_img)
        buffer = io.BytesIO(msg)
        try:
            # attempt to open the image with PIL
            image = Image.open(buffer)
        except PIL.UnidentifiedImageError:
            print("Could not identify the image format.")

        im = Image.fromarray(np.squeeze(image))
        return im

with gr.Blocks() as app:
    prompt = gr.Textbox(label="Prompt")
    submit_btn = gr.Button("Generate")
    img_output = gr.Image().style(height=512)
    submit_btn.click(fn=generate, inputs=prompt, outputs=img_output)

app.launch(server_name="0.0.0.0", debug=True, share=True)