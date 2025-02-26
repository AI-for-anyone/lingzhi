from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import sys
import os
import asyncio
from typing import Optional, Dict, Any

# Add parent directory to Python path to import core modules
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.utils import asr, vad, llm, tts
from config.logger import setup_logging

app = FastAPI()
logger = setup_logging()

# Global instances
instances: Dict[str, Any] = {}

class ProcessRequest(BaseModel):
    audio_data: bytes
    config: dict

class TextRequest(BaseModel):
    text: str
    config: dict

@app.post("/init")
async def initialize(config: dict):
    """Initialize all processing instances with the given configuration"""
    global instances
    try:
        instances['vad'] = vad.create_instance(
            config["selected_module"]["VAD"],
            config["VAD"][config["selected_module"]["VAD"]]
        )
        instances['asr'] = asr.create_instance(
            config["selected_module"]["ASR"]
            if 'type' not in config["ASR"][config["selected_module"]["ASR"]]
            else config["ASR"][config["selected_module"]["ASR"]]["type"],
            config["ASR"][config["selected_module"]["ASR"]],
            config["delete_audio"]
        )
        instances['llm'] = llm.create_instance(
            config["selected_module"]["LLM"]
            if 'type' not in config["LLM"][config["selected_module"]["LLM"]]
            else config["LLM"][config["selected_module"]["LLM"]]['type'],
            config["LLM"][config["selected_module"]["LLM"]]
        )
        instances['tts'] = tts.create_instance(
            config["selected_module"]["TTS"]
            if 'type' not in config["TTS"][config["selected_module"]["TTS"]]
            else config["TTS"][config["selected_module"]["TTS"]]['type'],
            config["TTS"][config["selected_module"]["TTS"]]
        )
        return {"status": "success", "message": "All instances initialized successfully"}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/vad")
async def process_vad(request: ProcessRequest):
    """Process audio with VAD"""
    if 'vad' not in instances:
        raise HTTPException(status_code=500, detail="VAD instance not initialized")
    try:
        result = instances['vad'].process(request.audio_data, request.config)
        return {"status": "success", "result": str(result).lower()}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/asr")
async def process_asr(request: ProcessRequest):
    """Process audio with ASR"""
    if 'asr' not in instances:
        raise HTTPException(status_code=500, detail="ASR instance not initialized")
    try:
        result = instances['asr'].process(request.audio_data, request.config)
        return {"status": "success", "result": result}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/llm")
async def process_llm(request: TextRequest):
    """Process text with LLM"""
    if 'llm' not in instances:
        raise HTTPException(status_code=500, detail="LLM instance not initialized")
    try:
        result = instances['llm'].process(request.text, request.config)
        return {"status": "success", "result": result}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/tts")
async def process_tts(request: TextRequest):
    """Process text with TTS"""
    if 'tts' not in instances:
        raise HTTPException(status_code=500, detail="TTS instance not initialized")
    try:
        result = instances['tts'].process(request.text, request.config)
        return {"status": "success", "result": result}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8001)
