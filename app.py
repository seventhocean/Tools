import subprocess
import threading
import time
from flask import Flask, request, Response, jsonify, send_file
import json
import os

app = Flask(__name__)

# 构建任务状态
build_status = {}

def run_build_task(task_id, current_version, target_version):
    """在后台运行构建任务"""
    try:
        # 更新状态
        build_status[task_id] = {"percent": 0, "message": "开始构建过程"}
        
        # 步骤1: 获取镜像名称列表 (模拟)
        build_status[task_id] = {"percent": 20, "message": "获取镜像列表"}
        # 这里可以调用您的实际脚本
        # subprocess.run(["cat", "/root/patch_image_tag_list.txt"], check=True)
        time.sleep(2)
        
        # 步骤2: 获取镜像并保存
        build_status[task_id] = {"percent": 50, "message": "拉取并保存镜像"}
        # 这里调用您的pull_save.sh脚本
        # subprocess.run(["/bin/bash", "/path/to/pull_save.sh"], check=True)
        time.sleep(3)
        
        # 步骤3: 创建升级包
        build_status[task_id] = {"percent": 80, "message": "创建升级包"}
        # 这里添加创建升级包的逻辑
        time.sleep(2)
        
        # 完成
        build_status[task_id] = {
            "percent": 100, 
            "message": "构建完成", 
            "complete": True,
            "download_url": f"/download/{task_id}"
        }
        
    except Exception as e:
        build_status[task_id] = {
            "percent": 0, 
            "message": f"构建失败: {str(e)}", 
            "complete": True,
            "error": True
        }

@app.route('/')
def index():
    return send_file('index.html')

@app.route('/build')
def build():
    current = request.args.get('current')
    target = request.args.get('target')
    task_id = f"build_{int(time.time())}"
    
    # 启动后台构建任务
    thread = threading.Thread(
        target=run_build_task, 
        args=(task_id, current, target)
    )
    thread.start()
    
    def generate():
        last_percent = -1
        while True:
            if task_id in build_status:
                status = build_status[task_id]
                
                # 只在进度更新时发送消息
                if status["percent"] != last_percent:
                    last_percent = status["percent"]
                    yield f"data: {json.dumps(status)}\n\n"
                
                # 如果任务完成，退出循环
                if status.get("complete"):
                    if status.get("error"):
                        yield f"data: {json.dumps({'status': 'error', 'message': status['message']})}\n\n"
                    else:
                        yield f"data: {json.dumps({'status': 'complete', 'download_url': status['download_url']})}\n\n"
                    break
            time.sleep(1)
        
        yield "event: close\ndata: \n\n"
    
    return Response(generate(), mimetype='text/event-stream')

@app.route('/download/<task_id>')
def download(task_id):
    # 这里返回构建好的升级包文件
    # 假设升级包文件位于 /root/upgrade_package.zip
    return send_file(
        "/root/upgrade_package.zip",
        as_attachment=True,
        download_name=f"upgrade_package_{task_id}.zip"
    )

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000, debug=True)