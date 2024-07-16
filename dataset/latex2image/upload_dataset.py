from huggingface_hub import HfApi, delete_repo

# pip install huggingface_hub
# huggingface-cli login

api = HfApi()

local_dir = "./output"
repo_id = "yejibing/table"

# delete_repo(repo_id=repo_id, repo_type="dataset")
api.create_repo(repo_id=repo_id, repo_type="dataset", exist_ok=True)
api.upload_folder(
    folder_path=local_dir,
    repo_id=repo_id,
    repo_type="dataset",
)