from abc import ABC, abstractmethod
from config.logger import setup_logging
import opuslib_next
import time
import numpy as np
import torch
import traceback

TAG = __name__
logger = setup_logging()

class VAD(ABC):
    @abstractmethod
    def is_vad(self, data):
        """检测音频数据中的语音活动"""
        pass


class SileroVAD(VAD):
    def __init__(self, config):
        logger.bind(tag=TAG).info("SileroVAD", config)
        self.model, self.utils = torch.hub.load(repo_or_dir=config["model_dir"],
                                                source='local',
                                                model='silero_vad',
                                                force_reload=False)

        logger.bind(tag=TAG).info("SileroVAD model init done")
        (get_speech_timestamps, _, _, _, _) = self.utils

        self.decoder = opuslib_next.Decoder(16000, 1)
        self.vad_threshold = config.get("threshold", 0.5)  # 默认阈值为0.5
        self.silence_threshold_ms = config.get("min_silence_duration_ms", 500)  # 默认静默阈值为500ms
        self.sample_rate = config.get("sample_rate", 16000)  # 默认采样率为16kHz
        self.frame_size = config.get("frame_size", 512)  # 默认帧大小为512样本

        logger.bind(tag=TAG).info(f"SileroVAD initialized with threshold={self.vad_threshold}, "
                                 f"silence_threshold={self.silence_threshold_ms}ms, "
                                 f"sample_rate={self.sample_rate}Hz, "
                                 f"frame_size={self.frame_size} samples")

    def is_vad(self, audio_data, config=None):
        """
        检测音频数据中的语音活动
        
        参数:
            audio_data: 二进制音频数据
            config: 可选的配置参数，可以覆盖实例化时的配置
            
        返回:
            bool: 是否检测到语音活动
        """
        try:
            # logger.bind(tag=TAG).debug(f"处理音频数据: 长度={len(audio_data)}字节, "
            #                           f"采样率={self.sample_rate}Hz, "
            #                           f"帧大小={self.frame_size}样本, "
            #                           f"阈值={self.vad_threshold}")
            
            # 转换为 int16 数组
            audio_int16 = np.frombuffer(audio_data, dtype=np.int16)
            
            # logger.bind(tag=TAG).debug(f"转换后样本数: {len(audio_int16)}")
            
            # 转换为浮点数并归一化
            audio_float32 = audio_int16.astype(np.float32) / 32768.0
            
            # 转换为张量
            audio_tensor = torch.from_numpy(audio_float32)
            
            # 确保张量形状正确
            if audio_tensor.ndim == 1:
                audio_tensor = audio_tensor.unsqueeze(0)  # 添加批次维度
            
            # 检测语音活动
            speech_prob = self.model(audio_tensor, self.sample_rate).item()
            client_have_voice = (speech_prob >= self.vad_threshold)
            
            # logger.bind(tag=TAG).debug(f"语音检测概率: {speech_prob:.4f}, 阈值: {self.vad_threshold}, 结果: {client_have_voice}")
            
            return client_have_voice
        except Exception as e:
            logger.bind(tag=TAG).error(f"处理音频数据时出错: {str(e)}")
            logger.bind(tag=TAG).error(traceback.format_exc())
            # 出错时默认返回 False
            return False

def create_instance(class_name, *args, **kwargs) -> VAD:
    # 获取类对象
    cls_map = {
        "SileroVAD": SileroVAD,
        # 可扩展其他SileroVAD实现
    }

    if cls := cls_map.get(class_name):
        return cls(*args, **kwargs)
    raise ValueError(f"不支持的SileroVAD类型: {class_name}")