#!/usr/bin/env python3
# Copyright (c) 2022, NVIDIA CORPORATION. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Client for Stable Diffusion 1.5."""
import gradio as gr

import argparse
import base64
import io
import logging
import pathlib

import numpy as np
from PIL import Image  # pytype: disable=import-error

from pytriton.client import ModelClient

logger = logging.getLogger("examples.huggingface_stable_diffusion.client")

parser = argparse.ArgumentParser()
parser.add_argument(
        "--url",
        default="localhost",
        help=(
            "Url to Triton server (ex. grpc://localhost:8001)."
            "HTTP protocol with default port is used if parameter is not provided"
        ),
        required=False,
    )
parser.add_argument(
        "--init-timeout-s",
        type=float,
        default=600.0,
        help="Server and model ready state timeout in seconds",
        required=False,
    )
parser.add_argument(
        "--iterations",
        type=int,
        default=1,
        help="Number of requests per client.",
        required=False,
    )
parser.add_argument(
        "--results-path",
        type=str,
        default="results",
        help="Path to folder where images should be stored.",
        required=False,
    )
parser.add_argument(
        "--verbose",
        action="store_true",
        default=False,
    )
args = parser.parse_args()
log_level = logging.DEBUG if args.verbose else logging.INFO
logging.basicConfig(level=log_level, format="%(asctime)s - %(levelname)s - %(name)s: %(message)s")

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
