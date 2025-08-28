import subprocess
import threading
import time
from flask import Flask, request, Response, jsonify, send_file
import json
import os
import glob

app = Flask(__name__)

# 构建任务状态
build_status = {}

def run_build_task(task_id, current_version, target_version):
    """在后台运行构建任务"""
    try:
        # 更新状态
        build_status[task_id] = {"percent": 0, "message": "开始构建过程"}
        
        # 步骤1: 获取镜像名称列表
        build_status[task_id] = {"percent": 20, "message": "获取镜像列表"}
        subprocess.run(["cat", "/home/auto_packing_no_delete/patch_image_tag_list.txt"], check=True)
        time.sleep(1)
        
        # 步骤2: 获取镜像并保存
        build_status[task_id] = {"percent": 50, "message": "拉取并保存镜像"}
        # 这里调用您的pull_save.sh脚本
        subprocess.run(["/bin/bash", "/home/auto_packing_no_delete/pull_save.sh"], check=True)
        time.sleep(2)
        
        # 步骤3: 创建升级包（打包所有.tar文件）
        build_status[task_id] = {"percent": 80, "message": "创建升级包"}
        
        # 切换到目标目录
        os.chdir("/home/auto_packing_no_delete/")
        
        # 获取所有.tar文件
        tar_files = glob.glob("*.tar")
        if not tar_files:
            raise Exception("未找到任何.tar文件")
        
        # 创建升级包（将所有.tar文件打包成zip）
        upgrade_package_name = f"upgrade_package_{current_version}_to_{target_version}_{task_id}.zip"
        subprocess.run(["zip", "-j", upgrade_package_name] + tar_files, check=True)
        
        # 直接使用当前目录下的zip文件路径
        final_path = os.path.join("/home/auto_packing_no_delete/", upgrade_package_name)
        
        # 检查文件是否创建成功
        if not os.path.exists(final_path):
            raise Exception(f"升级包文件创建失败: {final_path}")
        
        # 完成
        build_status[task_id] = {
            "percent": 100, 
            "message": f"构建完成，包含 {len(tar_files)} 个.tar文件", 
            "complete": True,
            "download_url": f"/download/{task_id}",
            "package_path": final_path
        }
        
    except subprocess.CalledProcessError as e:
        build_status[task_id] = {
            "percent": 0, 
            "message": f"构建失败: 命令执行错误 - {str(e)}", 
            "complete": True,
            "error": True
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
                        # 先定义字典
                        response_data = {
                            'status': 'complete', 
                            'download_url': status['download_url'],
                            'message': status['message']
                        }
                        # 再格式化字符串
                        yield f"data: {json.dumps(response_data)}\n\n"
                    break
            time.sleep(0.5)
        
        yield "event: close\ndata: \n\n"
    
    return Response(generate(), mimetype='text/event-stream')

@app.route('/download/<task_id>')
def download(task_id):
    if task_id in build_status and 'package_path' in build_status[task_id]:
        package_path = build_status[task_id]['package_path']
        if os.path.exists(package_path):
            return send_file(
                package_path,
                as_attachment=True,
                download_name=os.path.basename(package_path)
            )
    
    return "文件不存在或已过期", 404

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8000, debug=True)