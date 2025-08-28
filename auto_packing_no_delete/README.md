deepflow-upgrade-builder/
├── app.py                  # 后端核心服务（Flask）：接口、构建逻辑、下载管理
├── index.html              # 前端页面：版本选择、构建进度展示、升级包下载
├── pull_save.sh            # 镜像拉取脚本：支持Docker/Nerdctl，从列表/命令行拉取
├── oss_patch_processor.sh  # OSS同步脚本：定期下载补丁包、提取镜像列表
├── image_tar/              # 镜像存储&升级包输出目录（自动创建）
│   └── upgrade_xxx.zip     # 生成的升级包（示例）
├── latest_image_list/      # 最新镜像列表目录（自动创建，OSS脚本生成）
│   └── patch_image_tag_list.txt  # 核心镜像列表文件
└── logs/                   # 日志目录（自动创建，所有流程日志）
    ├── app.log             # 后端服务日志
    ├── pull_save.log       # 镜像拉取日志
    └── oss_processor.log   # OSS同步日志