#!/usr/bin/env python3
"""
编码转换工具模块
用于处理Calibre数据库中的GBK编码问题
"""
import logging
import chardet

logger = logging.getLogger(__name__)

def convert_gbk_to_utf8(text):
    """
    将GBK/Big5编码的文本转换为UTF-8
    支持简体中文(GBK)和繁体中文(Big5)
    如果转换失败，返回原始文本
    """
    if not text:
        return text
    
    try:
        # 首先尝试直接作为UTF-8处理
        if isinstance(text, str):
            # 检查是否包含乱码特征（如连续的问号或方框）
            if '�' in text or '□' in text:
                # 尝试重新解码
                try:
                    # 尝试将字符串编码为latin-1，然后解码为GBK
                    gbk_bytes = text.encode('latin-1')
                    # 先尝试GBK
                    try:
                        decoded_text = gbk_bytes.decode('gbk', errors='strict')
                        return decoded_text
                    except UnicodeDecodeError:
                        # 如果GBK失败，尝试Big5（繁体中文）
                        try:
                            decoded_text = gbk_bytes.decode('big5', errors='strict')
                            return decoded_text
                        except UnicodeDecodeError:
                            # 如果都失败，使用replace模式
                            decoded_text = gbk_bytes.decode('gbk', errors='replace')
                            return decoded_text
                except (UnicodeEncodeError, UnicodeDecodeError):
                    # 如果失败，可能是已经是正确的UTF-8
                    return text
            else:
                # 看起来已经是正常的UTF-8文本
                return text
                
        elif isinstance(text, bytes):
            # 如果是字节串，先检测编码
            detected = chardet.detect(text)
            if detected['encoding']:
                encoding = detected['encoding'].lower()
                if 'gb' in encoding:
                    return text.decode(detected['encoding'], errors='replace')
                elif 'big5' in encoding or 'cp950' in encoding:
                    return text.decode('big5', errors='replace')
                else:
                    # 使用检测到的编码
                    return text.decode(detected['encoding'], errors='replace')
            else:
                # 编码检测失败，依次尝试GBK和Big5
                try:
                    return text.decode('gbk', errors='strict')
                except UnicodeDecodeError:
                    try:
                        return text.decode('big5', errors='strict')
                    except UnicodeDecodeError:
                        # 如果都失败，使用GBK的replace模式
                        return text.decode('gbk', errors='replace')
    except Exception as e:
        logger.warning(f"编码转换失败: {e}, 返回原始文本")
        return text

def safe_convert_text(text):
    """
    安全的文本转换函数
    处理各种边界情况
    """
    if text is None:
        return ""
    
    if not isinstance(text, (str, bytes)):
        return str(text)
    
    return convert_gbk_to_utf8(text)

def convert_dict_values(data_dict, fields_to_convert):
    """
    转换字典中指定字段的编码
    :param data_dict: 要转换的字典
    :param fields_to_convert: 需要转换的字段列表
    :return: 转换后的字典
    """
    if not isinstance(data_dict, dict):
        return data_dict
    
    result = data_dict.copy()
    for field in fields_to_convert:
        if field in result and result[field] is not None:
            result[field] = safe_convert_text(result[field])
    
    return result

def convert_list_values(data_list, fields_to_convert):
    """
    转换列表中字典的指定字段编码
    :param data_list: 包含字典的列表
    :param fields_to_convert: 需要转换的字段列表
    :return: 转换后的列表
    """
    if not isinstance(data_list, list):
        return data_list
    
    result = []
    for item in data_list:
        if isinstance(item, dict):
            result.append(convert_dict_values(item, fields_to_convert))
        else:
            result.append(item)
    
    return result