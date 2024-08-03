import json
import os

def validate_dataset(metadata_file_path):
    valid_lines = []
    directory = os.path.dirname(metadata_file_path)
    
    # 读取 metadata.jsonl 文件
    with open(metadata_file_path, 'r') as file:
        for line in file:
            try:
                # 解析 JSON 对象
                json_obj = json.loads(line.strip())
                
                # 获取 file_name 字段的值
                file_name = json_obj.get('file_name')
                
                if file_name:
                    # 构建完整的文件路径
                    full_file_path = os.path.join(directory, file_name)
                    
                    if os.path.exists(full_file_path):
                        # 如果文件存在，保留该行
                        valid_lines.append(line)
                    else:
                        print(f"file is not exist: {full_file_path}")
                else:
                    print("file_name field miss")
            
            except json.JSONDecodeError:
                print(f"not vaild JSON line: {line}")
    
    # 将有效的行写回文件
    with open(metadata_file_path, 'w') as file:
        file.writelines(valid_lines)
    
    print(f"处理完成。保留了 {len(valid_lines)} 行有效数据。")

# 使用函数
metadata_file_path = '/home/yejibing/code/doc-train/dataset/latex2image/output/train/metadata.jsonl'
validate_dataset(metadata_file_path)