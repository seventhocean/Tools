import subprocess
import threading
import time
from flask import Flask, request, Response, jsonify, send_file
import json
import os
import glob
import re

# 初始化Flask应用（适配当前目录结构）
app = Flask(__name__, static_folder='.', static_url_path='')

# 构建任务状态（全局变量）
build_status = {}

def get_oss_versions():
    """从OSS获取版本列表（适配Python 3.6）"""
    try:
        # 执行ossutil命令（Python 3.6不支持text参数，使用universal_newlines）
        result = subprocess.run(
            ["ossutil", "ls", "oss://df-patch-no-delete/patch/6.6/6.6.9/latest/"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True,  # 替代text=True
            check=True
        )
        
        versions = []
        seen_dates = set()
        
        for line in result.stdout.split('\n'):
            if '.tar.gz' in line:
                # 正则匹配版本日期（格式：08-20250519）
                match = re.search(r'(\d{2}-\d{8})-\d{5}-ALL\.tar\.gz', line)
                if match:
                    version_str = match.group(1)
                    date_str = version_str.split('-')[1]  # 提取纯日期：20250519
                    
                    if date_str not in seen_dates:
                        seen_dates.add(date_str)
                        versions.append({
                            'value': version_str,
                            'display': version_str,
                            'date': date_str
                        })
        
        # 按日期升序排序
        versions.sort(key=lambda x: x['date'])
        return versions
        
    except subprocess.CalledProcessError as e:
        print(f"OSS命令执行失败: {e.stderr}")
        return []
    except Exception as e:
        print(f"解析版本数据失败: {str(e)}")
        return []

def run_build_task(task_id, current_version, target_version):
    """后台构建任务（适配Python 3.6）"""
    try:
        # 构建根目录（当前项目根目录）
        root_dir = os.path.dirname(os.path.abspath(__file__))
        image_tar_dir = os.path.join(root_dir, 'image_tar')
        
        # 初始化状态
        build_status[task_id] = {
            "status": "progress",
            "percent": 0, 
            "message": "初始化构建任务"
        }
        time.sleep(1)
        
        # 步骤1：校验镜像列表文件
        image_list_path = os.path.join(root_dir, 'patch_image_tag_list.txt')
        if not os.path.exists(image_list_path):
            raise Exception(f"镜像列表文件不存在: {image_list_path}")
        build_status[task_id] = {
            "status": "progress",
            "percent": 20, 
            "message": "读取镜像列表文件"
        }
        subprocess.run(
            ["cat", image_list_path], 
            check=True, 
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True
        )
        time.sleep(1)
        
        # 步骤2：执行拉取脚本（输出到image_tar）
        build_status[task_id] = {
            "status": "progress",
            "percent": 50, 
            "message": "拉取镜像并保存"
        }
        pull_script_path = os.path.join(root_dir, 'pull_save.sh')
        if not os.path.exists(pull_script_path):
            raise Exception(f"拉取脚本不存在: {pull_script_path}")
        
        # 执行脚本（传递目录参数）
        pull_result = subprocess.run(
            ["/bin/bash", pull_script_path, image_tar_dir],
            check=True, 
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True  # 替代text=True
        )
        print(f"拉取脚本输出: {pull_result.stdout}")
        time.sleep(2)
        
        # 步骤3：打包升级包（从image_tar目录取文件）
        build_status[task_id] = {
            "status": "progress",
            "percent": 80, 
            "message": "打包升级包"
        }
        os.chdir(image_tar_dir)  # 切换到image_tar目录
        
        # 获取所有.tar文件
        tar_files = glob.glob("*.tar")
        if not tar_files:
            raise Exception(f"image_tar目录中无.tar文件: {image_tar_dir}")
        
        # 生成升级包（存放在image_tar目录）
        upgrade_package_name = f"upgrade_{current_version}_to_{target_version}_{task_id}.zip"
        upgrade_package_path = os.path.join(image_tar_dir, upgrade_package_name)
        
        # 执行打包
        zip_result = subprocess.run(
            ["zip", "-j", upgrade_package_path] + tar_files, 
            check=True, 
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            universal_newlines=True  # 替代text=True
        )
        print(f"打包输出: {zip_result.stdout}")
        
        # 校验文件存在
        if not os.path.exists(upgrade_package_path):
            raise Exception(f"升级包创建失败: {upgrade_package_path}")
        
        # 构建完成状态
        build_status[task_id] = {
            "status": "complete",
            "percent": 100, 
            "message": f"构建成功！包含 {len(tar_files)} 个镜像文件", 
            "complete": True,
            "download_url": f"/download/{task_id}",
            "package_path": upgrade_package_path,
            "package_name": upgrade_package_name
        }
        
    except subprocess.CalledProcessError as e:
        error_msg = f"命令执行失败: {e.cmd} → {e.stderr.strip()}"
        build_status[task_id] = {
            "status": "error",
            "percent": 0, 
            "message": error_msg, 
            "complete": True,
            "error": True
        }
    except Exception as e:
        error_msg = str(e)
        build_status[task_id] = {
            "status": "error",
            "percent": 0, 
            "message": error_msg, 
            "complete": True,
            "error": True
        }

# 根路径：返回index.html
@app.route('/')
def index():
    return send_file('index.html')

# 获取版本列表接口
@app.route('/versions')
def versions():
    try:
        versions = get_oss_versions()
        return jsonify({
            'success': True,
            'versions': versions
        })
    except Exception as e:
        return jsonify({
            'success': False,
            'message': str(e)
        })

# 构建任务接口（SSE实时推送）
@app.route('/build')
def build():
    current_version = request.args.get('current')
    target_version = request.args.get('target')
    
    if not current_version or not target_version:
        return jsonify({
            'success': False,
            'message': "当前版本和目标版本不能为空"
        }), 400
    
    task_id = f"task_{int(time.time())}"
    # 启动后台线程（Python 3.6中daemon参数需显式设置）
    build_thread = threading.Thread(
        target=run_build_task, 
        args=(task_id, current_version, target_version),
        daemon=True
    )
    build_thread.start()
    
    # SSE响应生成器
    def sse_generator():
        last_percent = -1
        while True:
            if task_id not in build_status:
                time.sleep(0.3)
                continue
            
            current_status = build_status[task_id]
            if current_status["percent"] != last_percent or current_status["status"] in ["complete", "error"]:
                last_percent = current_status["percent"]
                yield f"data: {json.dumps(current_status)}\n\n"
            
            if current_status.get("complete"):
                yield "event: close\ndata: 任务结束\n\n"
                break
            
            time.sleep(0.5)
    
    return Response(
        sse_generator(),
        mimetype='text/event-stream',
        headers={
            "Cache-Control": "no-cache",
            "Connection": "keep-alive"
        }
    )

# 下载接口（从image_tar目录取文件）
@app.route('/download/<task_id>')
def download(task_id):
    if task_id not in build_status:
        return "任务不存在", 404
    
    task_status = build_status[task_id]
    if task_status.get("status") != "complete" or "package_path" not in task_status:
        return "升级包未就绪", 400
    
    package_path = task_status["package_path"]
    package_name = task_status.get("package_name", "upgrade.zip")
    
    if not os.path.exists(package_path):
        return "文件已删除", 404
    
    return send_file(
        package_path,
        as_attachment=True,
        download_name=package_name,
        mimetype='application/zip'
    )

if __name__ == '__main__':
    # 确保image_tar目录存在
    image_tar_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'image_tar')
    if not os.path.exists(image_tar_dir):
        os.makedirs(image_tar_dir)
    
    # 启动服务（Python 3.6兼容写法）
    app.run(host='0.0.0.0', port=8000, debug=True)
