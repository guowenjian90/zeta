# Clone the pyTriton repo
git clone https://github.com/triton-inference-server/pytriton

# How to create a PyTriton environment
1. You can build your own environment or lanch the NVIDIA PyTorch Docker container
docker run --gpus all -it --shm-size=256m --rm --net=host -v ${PWD}:/workspace/ -v ${PWD}/pytriton/examples nvcr.io/nvidia/pytorch:23.03-py3 bash

# Install PyTriton libary
pip install -U nvidia-pytriton

# There are several examples provided and you can go into the corresponding folders (We use a simple example here)
cd pytriton/examples/add_sub_python




# Some notes
use --shm-size=256m in launching docker or there will be an error "Stub process is unhealthy and it will be restarted"