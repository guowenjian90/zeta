# Pytriton 
[PyTriton](https://github.com/triton-inference-server/pytriton#examples) is a Flask/FastAPI-like interface that simplifies Triton's deployment in Python environments. The library allows serving Machine Learning models directly from Python through NVIDIA's Triton Inference Server.


# How to deploy Stable Diffusion with pyTrition

Step 1: Clone the pyTriton repo to get examples

```
git clone https://github.com/triton-inference-server/pytriton
```

Step 2: create a PyTriton environment
You can build your own environment or lanch the NVIDIA PyTorch Docker container

```
docker run --gpus all -it --shm-size=8gb --rm --net=host -v ${PWD}:/workspace/ -v ${PWD}/pytriton nvcr.io/nvidia/pytorch:23.03-py3 bash
```
Step 3: Install libary
```
./install.sh
```

Step 4: Launch Server

```
./server.py
```

Step 5: Launch Server

```
./client.py
```

## Notes
use --shm-size = 8gb in launching docker or there will be an error "Stub process is unhealthy and it will be restarted"
