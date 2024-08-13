#/usr/bin/env python3
# -*- coding:utf-8 -*-
# author: fgj
# date: 2024/07/18
# description: 从https://webstatic.h3yun.com/lib/monaco-editor.json获取下载url，下载文件并上传到OSS


import argparse
import hashlib
import os
import shutil
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import oss2
import requests
from loguru import logger
from requests.adapters import HTTPAdapter
from tenacity import retry, stop_after_attempt, wait_fixed
from urllib3.util.retry import Retry


def configure_logger():
    '''初始化日志配置'''
    log_format = "<green>{time:YYYY-MM-DD HH:mm:ss}</green> | <level>{level}</level> | <cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> - <white>{message}</white>"
    log_level = "INFO"
    log_file = f"{os.path.dirname(os.path.abspath(__file__))}/app_{{time:YYYYMMDD}}.log"
    # 移除默认的日志配置
    logger.remove()
    # 控制台日志输出格式  
    logger.add(sys.stdout, level=log_level, colorize=True, format=log_format)
    # 文件日志记录格式
    logger.add(log_file, rotation="1 day", retention="3 days", level=log_level, format=log_format, enqueue=True)


class OSSUploader:

    def __init__(self, access_key_id, access_key_secret, endpoint, bucket_name, visit_url, base_url, new_directory_name, max_workers):
        self.access_key_id = access_key_id
        self.access_key_secret = access_key_secret
        self.endpoint = endpoint
        self.bucket_name = bucket_name
        
        self.visit_url = visit_url
        self.base_url = base_url
        self.new_directory_name = new_directory_name
        self.successful_upload = 0
        self.max_workers = max_workers
        self.flag = False
        self.current_directory = os.path.dirname(os.path.abspath(__file__))

        # 创建requests会话，增加重试机制
        self.session = requests.Session()
        session_retry = Retry(total=2, backoff_factor=0.5, status_forcelist=[500, 502, 503, 504])
        adapter = HTTPAdapter(max_retries=session_retry)
        self.session.mount('http://', adapter)
        self.session.mount('https://', adapter)

    def __enter__(self):
        """_summary_
        进入上下文管理器时, 初始化日志配置、初始化OSS、创建临时目录
        :return: _description_
        :rtype: _type_
        """
        # 初始化日志配置
        configure_logger()

        # 创建 OSS 认证对象和 Bucket 对象，并进行检查
        self.auth = oss2.Auth(self.access_key_id, self.access_key_secret)
        self.bucket = oss2.Bucket(self.auth, self.endpoint, self.bucket_name)
        try:
            self.bucket.get_bucket_info()
            logger.info(f"Bucket '{self.bucket_name}' exists")
        except oss2.exceptions.NoSuchBucket:
            logger.error(f"Bucket '{self.bucket_name}' does not exist")
            sys.exit(1)
        except Exception as e:
            logger.error(f"Failed to check bucket '{self.bucket_name}': {e}")
            sys.exit(1)

        # 创建临时目录
        self.new_directory_path = os.path.join(self.current_directory, self.new_directory_name)
        os.makedirs(self.new_directory_path, exist_ok=True)
        logger.debug(f"Directory '{self.new_directory_path}' created successfully")
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """_summary_
        退出上下文管理器时, 如果存在临时目录并且下载和上传的文件数一样, 就删除临时目录
        """      
        if self.flag and os.path.exists(self.new_directory_path):
            shutil.rmtree(self.new_directory_path)
            logger.info(f"The upload and download are consistent, and the operation is successful, so delete directory '{self.new_directory_path}'")
        else:
            logger.warning("Upload and download counts do not match")            

    def get_urls(self):
        """_summary_
        获取所有下载url
        :return: _description_
        :rtype: _type_
        """
        try:
            response = self.session.get(self.visit_url, timeout=5)
            # 请求失败会抛出异常
            response.raise_for_status()
            url_list = response.json()
            file_dict = {url.split('/')[-1]: url.replace(self.base_url, "") for url in url_list}
            logger.debug(f"url_list: {url_list}, file_dict: {file_dict}")
            return url_list, file_dict
        except requests.exceptions.Timeout:
            logger.error(f"Request to {self.visit_url} timed out")
        except requests.exceptions.RequestException as e:
            logger.error(f"Failed to fetch data from {self.visit_url}: {e}")

    def get_local_file_hash(self, file_path):
        """_summary_
        计算本地文件的MD5值
        :param file_path: _description_
        :type file_path: _type_
        :return: _description_
        :rtype: _type_
        """
        hash_md5 = hashlib.md5()
        with open(file_path, "rb") as f:
            # 每次读取 4096 字节, 就更新一次MD5值, 读取到文件末尾会返回一个空的字节串, 迭代器会停止迭代
            for chunk in iter(lambda: f.read(4096), b""):
                hash_md5.update(chunk)
        # 返回整个文件内容的MD5值
        return hash_md5.hexdigest()

    def get_remote_file_hash(self, url):
        """_summary_
        计算远程文件的MD5值
        :param url: _description_
        :type url: _type_
        :return: _description_
        :rtype: _type_
        """
        response = self.session.get(url)
        response.raise_for_status()
        hash_md5 = hashlib.md5(response.content).hexdigest()
        return hash_md5
    
    # 重试1次，重试间隔为1秒
    @retry(stop=stop_after_attempt(1), wait=wait_fixed(1))
    def download_file(self, url, file_dict):
        """_summary_
        下载文件
        :param url: _description_
        :type url: _type_
        :param file_dict: _description_
        :type file_dict: _type_
        """
        file_name = url.split('/')[-1]
        file_path = os.path.join(self.new_directory_path, file_name)
        oss_path = file_dict[file_name]

        # 文件存在且MD5值一致就跳过下载
        if os.path.exists(file_path) and os.path.getsize(file_path) > 0:
            local_hash = self.get_local_file_hash(file_path)
            remote_hash = self.get_remote_file_hash(url)       
            if local_hash == remote_hash:
                logger.debug(f"file_name: {file_name}, local_hash: {local_hash}, remote_hash: {remote_hash}")
                logger.debug(f"{file_name} already exists and is identical, skipping download")
                self.upload_to_oss(file_path, oss_path)
                return
                   
        try:                          
            response = self.session.get(url)
            response.raise_for_status()
            with open(file_path, 'wb') as f:
                f.write(response.content)
            logger.debug(f"Succeed to download {file_name}")
            self.upload_to_oss(file_path, oss_path)
        except Exception as e:
            logger.error(f"Failed to download {file_name}: {e}")
            raise

    @retry(stop=stop_after_attempt(1), wait=wait_fixed(1))
    def upload_to_oss(self, file_path, oss_path):
        """_summary_
        上传文件到OSS
        :param file_path: _description_
        :type file_path: _type_
        :param oss_path: _description_
        :type oss_path: _type_
        """
        try:
            with open(file_path, 'rb') as f:
                self.bucket.put_object(oss_path, f)
            self.successful_upload += 1
            logger.debug(f"Succeed to upload {file_path} to OSS at {oss_path}")
        except Exception as e:
            logger.error(f"Failed to upload {file_path} to OSS at {oss_path}: {e}")
            raise
    
    # 用于捕获并在日志中记录函数中的所有异常
    @logger.catch
    def run(self):
        start_time = time.time()

        # 获取url列表和文件映射
        url_list, file_dict = self.get_urls()
        
        # 多线程下载和上传文件
        with ThreadPoolExecutor(max_workers=self.max_workers) as exec:
            futures = [exec.submit(self.download_file, url, file_dict) for url in url_list]
            # 等待任务完成
            for future in as_completed(futures): 
                try:
                    future.result()
                except Exception as e:
                    logger.error(f"An error occurred during execution: {e}")

        cost_time = time.time() - start_time

        # 统计下载文件数
        if self.new_directory_path:
            # os.scandir会返回一个迭代器, 如果迭代对象是文件, 则值为1, 最后sum统计和
            file_count = sum(1 for obj in os.scandir(self.new_directory_path) if obj.is_file())
            logger.info(f"Total number of successful downloads: {file_count}, total number of successful uploads: {self.successful_upload}, cost time: {cost_time:.2f}s")
        
        # 标记完成状态
        if file_count == self.successful_upload == len(url_list):
            self.flag = True

    @staticmethod   
    def parse_args():
        parser = argparse.ArgumentParser(
            description="下载文件并上传到OSS, 需要传入4个参数: endpoint, bucket_name, access_key_id, access_key_secret \n"
                        "eg: python oss_uploader.py https://oss-cn-shenzhen.aliyuncs.com bucket_name access_key_id access_key_secret",
            formatter_class=argparse.RawTextHelpFormatter
        )
        parser.add_argument('endpoint', type=str, help='OSS endpoint, 对象存储访问地址')
        parser.add_argument('bucket_name', type=str, help='OSS bucket name, 对象存储桶名')
        parser.add_argument('access_key_id', type=str, help='AccessKey ID, 阿里云RAM访问控制api调用的id')
        parser.add_argument('access_key_secret', type=str, help='AccessKey Secret, 阿里云RAM访问控制api调用的secret')
        args = parser.parse_args()

        if len(sys.argv) != 5:
            parser.print_help()
            sys.exit(1)
        
        return args

if __name__ == "__main__":

    args = OSSUploader.parse_args()

    # URL
    VISIT_URL = "https://webstatic.h3yun.com/lib/monaco-editor.json"
    BASE_URL = "https://cdn.bootcdn.net/"
    # 临时目录
    NEW_DIRECTORY_NAME = "tmp_js"
    # 线程数
    MAX_WORKERS = min(5, max(1, os.cpu_count() // 3))
    
    with OSSUploader(args.access_key_id, args.access_key_secret, args.endpoint, args.bucket_name, VISIT_URL, BASE_URL, NEW_DIRECTORY_NAME, MAX_WORKERS) as o:
        o.run()