import asyncio
from config.logger import setup_logging
import os
import numpy as np
import opuslib_next
from pydub import AudioSegment
from abc import ABC, abstractmethod
import io

TAG = __name__
logger = setup_logging()


class TTSProviderBase(ABC):
    def __init__(self, config, delete_audio_file):
        self.delete_audio_file = delete_audio_file
        # self.output_file = config.get("output_file")

    def to_tts(self, text):
        try:
            origin_data = self.text_to_speak(text)
            data, duration = self.origin_data_to_opus_data(origin_data)
            logger.bind(tag=TAG).info(f"Generated data length: {len(data)}, text: {text[:]}")
            return data, duration
        except Exception as e:
            logger.bind(tag=TAG).info(f"Failed to generate TTS file: {e}")
            return None

    @abstractmethod
    def text_to_speak(self, text):
        pass

    def origin_data_to_opus_data(self, origin_data):

        # 二进制数据，直接从二进制数据创建 AudioSegment
        audio = AudioSegment.from_file(io.BytesIO(origin_data), format="wav")

        duration = len(audio) / 1000.0

        # 转换为单声道和16kHz采样率（确保与编码器匹配）
        audio = audio.set_channels(1).set_frame_rate(16000)

        # 获取原始PCM数据（16位小端）
        raw_data = audio.raw_data

        # 初始化Opus编码器
        encoder = opuslib_next.Encoder(16000, 1, opuslib_next.APPLICATION_AUDIO)

        # 编码参数
        frame_duration = 60  # 60ms per frame
        frame_size = int(16000 * frame_duration / 1000)  # 960 samples/frame

        opus_datas = []
        # 按帧处理所有音频数据（包括最后一帧可能补零）
        for i in range(0, len(raw_data), frame_size * 2):  # 16bit=2bytes/sample
            # 获取当前帧的二进制数据
            chunk = raw_data[i:i + frame_size * 2]

            # 如果最后一帧不足，补零
            if len(chunk) < frame_size * 2:
                chunk += b'\x00' * (frame_size * 2 - len(chunk))

            # 转换为numpy数组处理
            np_frame = np.frombuffer(chunk, dtype=np.int16)

            # 编码Opus数据
            opus_data = encoder.encode(np_frame.tobytes(), frame_size)
            opus_datas.append(opus_data)

        logger.bind(tag=TAG).info(f"opus_datas Generated data length: {len(opus_datas)}")
        return opus_datas, duration
