from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import sys
import os
import asyncio
from typing import Optional, Dict, Any
from fastapi.responses import StreamingResponse

# Add parent directory to Python path to import core modules
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from core.utils import asr, vad, llm, tts 
from core.utils.util import is_segment, get_string_no_punctuation_or_emoji
from config.logger import setup_logging
from config.settings import load_config

app = FastAPI()
logger = setup_logging()

# Global instances
instances: Dict[str, Any] = {}

# Initialize instances on startup
@app.on_event("startup")
async def startup_event():
    """Initialize all processing instances on startup"""
    global instances
    config = load_config()
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
            config["TTS"][config["selected_module"]["TTS"]],
            config["delete_audio"]
        )
        logger.bind(tag="startup").info("All instances initialized successfully on startup")
    except Exception as e:
        logger.error(f"Failed to initialize instances on startup: {str(e)}")
        raise e

class ProcessRequest(BaseModel):
    audio_data: str  # base64编码的音频数据
    config: dict

class ASRProcessRequest(BaseModel):
    audio_data: list[str]  # base64编码的音频数据
    config: dict

class TextRequest(BaseModel):
    text: str
    config: dict

class LLMRequest(BaseModel):
    dialogue: list[dict]
    config: dict

@app.post("/vad")
async def process_vad(request: ProcessRequest):
    """Process audio with VAD"""
    if 'vad' not in instances:
        raise HTTPException(status_code=500, detail="VAD instance not initialized")
    try:
        # 解码base64音频数据
        import base64
        import traceback
        
        # logger.bind(tag="vad_api").debug(f"接收VAD请求，配置: {request.config}")
        
        try:
            audio_data = base64.b64decode(request.audio_data)
            # logger.bind(tag="vad_api").debug(f"成功解码base64数据，长度: {len(audio_data)}字节")
        except Exception as e:
            logger.bind(tag="vad_api").error(f"Base64解码失败: {str(e)}")
            raise HTTPException(status_code=400, detail=f"Invalid base64 data: {str(e)}")
        
        # 将请求配置传递给VAD实例
        result = instances['vad'].is_vad(audio_data, request.config)
        # logger.bind(tag="vad_api").debug(f"VAD检测结果: {result}")
        
        return {"status": "success", "result": result}
    except Exception as e:
        logger.bind(tag="vad_api").error(f"VAD处理错误: {str(e)}")
        logger.bind(tag="vad_api").error(traceback.format_exc())
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/asr")
async def process_asr(request: ASRProcessRequest):
    """Process audio with ASR"""
    if 'asr' not in instances:
        raise HTTPException(status_code=500, detail="ASR instance not initialized")
    try:
        logger.bind(tag="asr_api").debug(f"接收ASR请求，配置: {request.config}")
        audio_data_decoded = []
        for data in request.audio_data:
            # 解码base64音频数据
            import base64
            import traceback
            
            try:
                audio_data = base64.b64decode(data)
                audio_data_decoded.append(audio_data)
                # logger.bind(tag="asr_api").debug(f"成功解码base64数据，长度: {len(audio_data)}字节")
            except Exception as e:
                logger.bind(tag="asr_api").error(f"Base64解码失败: {str(e)}")
                raise HTTPException(status_code=400, detail=f"Invalid base64 data: {str(e)}")
            
        # 将请求配置传递给ASR实例   
        result, file_path = await instances['asr'].speech_to_text(audio_data_decoded, request.config['SessionId'])
        logger.bind(tag="asr_api").debug(f"ASR处理结果: {result}")
        return {"status": "success", "text": result}
    except Exception as e:
        logger.bind(tag="asr_api").error(f"ASR处理错误: {str(e)}")
        logger.bind(tag="asr_api").error(traceback.format_exc())
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/llm")
async def process_llm(request: LLMRequest):
    """Process text with LLM and stream the results"""
    if 'llm' not in instances:
        raise HTTPException(status_code=500, detail="LLM instance not initialized")
    
    try:
        # 创建一个异步生成器来处理LLM的流式响应
        async def generate_response():
            try: 
                # 准备对话内容
                dialogue = []
                
                # 如果配置中包含系统提示，添加系统提示
                if "system_prompt" in request.config:
                    dialogue.append({"role": "system", "content": request.config["system_prompt"]})
                    
                # 准备对话内容
                dialogue += request.dialogue
                
                # 添加用户消息
                # dialogue.append({"role": "user", "content": request.text})
                
                # 记录请求信息
                session_id = request.config.get("SessionId", "default_session")
                logger.bind(tag="llm_api").info(f"处理LLM请求: session={session_id}, text={request.dialogue[:]}...")
                
                # 调用LLM处理文本，这会返回一个生成器
                llm_response_generator = instances['llm'].response(session_id, dialogue)
                
                # 跟踪是否收到了任何响应
                received_any_response = False
                full_response = []
                start = 0
                
                # 生成JSON行响应
                for chunk in llm_response_generator:
                    # 处理特殊字符，确保JSON格式正确
                    full_response.append(chunk)

                    if is_segment(chunk):
                        segment_text = "".join(full_response[start:])
                        segment_text = segment_text.replace('\\', '\\\\').replace('"', '\\"').replace('\n', '\\n')
                        segment_text = get_string_no_punctuation_or_emoji(segment_text)
                        if len(segment_text) > 0:
                            received_any_response = True
                            yield f'{{"status": "streaming", "chunk": "{segment_text}"}}\n'
                            start = len(full_response)

                # 处理剩余的响应
                if start < len(full_response):
                    segment_text = "".join(full_response[start:])
                    segment_text = segment_text.replace('\\', '\\\\').replace('"', '\\"').replace('\n', '\\n')
                    segment_text = get_string_no_punctuation_or_emoji(segment_text)
                    if len(segment_text) > 0:
                        received_any_response = True
                        yield f'{{"status": "streaming", "chunk": "{segment_text}"}}\n'

                # 记录完整响应
                if received_any_response:
                    logger.bind(tag="llm_api").info(f"LLM响应完成: session={session_id}, 长度={len(full_response)}")
                    logger.bind(tag="llm_api").debug(f"LLM完整响应: {full_response}...")
                else:
                    logger.bind(tag="llm_api").warning(f"LLM没有返回任何响应: session={session_id}")
                    yield '{"status": "warning", "message": "未收到LLM响应"}\n'
                
                # 发送完成信号
                res = "".join(full_response[:])
                yield f'{{"status": "complete", "message": "{res}"}}\n'
                
            except Exception as e:
                import traceback
                error_msg = str(e)
                logger.bind(tag="llm_api").error(f"LLM处理错误: {error_msg}")
                logger.bind(tag="llm_api").error(traceback.format_exc())
                
                # 处理错误消息中的特殊字符
                error_msg_escaped = error_msg.replace('\\', '\\\\').replace('"', '\\"').replace('\n', '\\n')
                yield f'{{"status": "error", "message": "{error_msg_escaped}"}}\n'
        
        # 返回流式响应
        return StreamingResponse(
            generate_response(),
            media_type="application/x-ndjson"  # 使用换行分隔的JSON格式
        )
        
    except Exception as e:
        import traceback
        logger.bind(tag="llm_api").error(f"LLM API错误: {str(e)}")
        logger.bind(tag="llm_api").error(traceback.format_exc())
        raise HTTPException(status_code=500, detail=str(e))

@app.post("/tts")
async def process_tts(request: TextRequest):
    """Process text with TTS and return base64 encoded audio data"""
    if 'tts' not in instances:
        raise HTTPException(status_code=500, detail="TTS instance not initialized")
    try:
        logger.bind(tag="tts_api").debug(f"接收TTS请求，文本: {request.text[:]}...")
        
        # 调用 TTS 处理文本
        opus_data, duration = instances['tts'].to_tts(request.text)
        
        if opus_data is None:
            logger.bind(tag="tts_api").error(f"TTS处理失败: 未能生成音频")
            raise HTTPException(status_code=500, detail="Failed to generate TTS audio")
        
        # 将 opus 数据编码为 base64
        import base64
        
        # 将每个 opus 帧编码为 base64 并添加到列表中
        base64_frames = []
        for frame in opus_data:
            # 编码每一帧
            frame_base64 = base64.b64encode(frame).decode('utf-8')
            base64_frames.append(frame_base64)
        
        logger.bind(tag="tts_api").debug(f"TTS处理成功，音频长度: {duration:.2f}秒，帧数: {len(base64_frames)}")
        
        # 返回 base64 编码的音频数据列表和持续时间
        return {
            "status": "success", 
            "audio_data": base64_frames,
            "duration": duration,
            "format": "opus",
            "frame_duration": 60  # 与 TTS 模块中的帧持续时间保持一致
        }
    except Exception as e:
        logger.bind(tag="tts_api").error(f"TTS处理错误: {str(e)}")
        import traceback
        logger.bind(tag="tts_api").error(traceback.format_exc())
        raise HTTPException(status_code=500, detail=str(e))

@app.get("/health")
async def check_healthy():
    return {"status": "success", "message": "I'm healthy!"}

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="127.0.0.1", port=8001)
