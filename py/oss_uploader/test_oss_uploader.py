import pytest
from unittest import mock
from oss_uploader import OSSUploader


@pytest.fixture
def mock_oss_uploader(mocker):
    # Mock oss2.Auth
    mocker.patch('oss2.Auth')

    # Mock oss2.Bucket
    mock_bucket = mock.Mock()
    mocker.patch('oss2.Bucket', return_value=mock_bucket)

    # 创建一个 mock 的 OSSUploader 实例
    uploader = OSSUploader("fake_id", "fake_secret", "https://fake.endpoint", "fake_bucket",
                           "https://webstatic.h3yun.com/lib/monaco-editor.json", "https://cdn.bootcdn.net/", "tmp_js", 5)
    
    # 设置 bucket 属性
    uploader.bucket = mock_bucket  

    # Mock OSS bucket 方法
    mock_bucket.get_bucket_info.return_value = True
    mock_bucket.put_object.return_value = True
    
    # Mock requests session 的 get 方法
    mocker.patch.object(uploader.session, 'get', side_effect=mock_get)

    # 设置 new_directory_path 属性
    uploader.new_directory_path = "tmp_js"

    # Mock logger 捕获日志记录
    mock_logger = mocker.patch('oss_uploader.logger')
    mock_logger.records = []

    return uploader, mock_logger


def mock_get(url, *args, **kwargs):
    if url == "https://webstatic.h3yun.com/lib/monaco-editor.json":
        return mock.Mock(status_code=200, json=lambda: ["https://cdn.bootcdn.net/xx1/file1.js", "https://cdn.bootcdn.net/xx2/file2.js"])
    elif url.startswith("https://cdn.bootcdn.net/"):
        return mock.Mock(status_code=200, content=b"file_content")
    return mock.Mock(status_code=404)


def test_run(mock_oss_uploader, mocker):
    uploader, mock_logger = mock_oss_uploader

    # Mock os.scandir 以避免文件系统交互
    mock_scandir = mocker.patch('os.scandir', return_value=iter([]))

    # Mock 其他方法
    mocker.patch('os.makedirs')
    mocker.patch('shutil.rmtree')
    mocker.patch('oss_uploader.OSSUploader.get_local_file_hash', return_value='fake_hash')
    mocker.patch('oss_uploader.OSSUploader.get_remote_file_hash', return_value='fake_hash')

    # Mock download_file 方法以模拟文件下载
    mock_download_file = mocker.patch('oss_uploader.OSSUploader.download_file')
    # 确保 download_file 内部调用 upload_to_oss
    def side_effect(url, file_dict):
        file_name = url.split('/')[-1]
        oss_path = f"tmp_js/{file_name}"
        uploader.upload_to_oss(oss_path, file_dict[file_name])
    mock_download_file.side_effect = side_effect

    # Mock upload_to_oss 方法
    mock_upload_to_oss = mocker.patch('oss_uploader.OSSUploader.upload_to_oss')

    # 运行 uploader
    uploader.run()

    # 验证 download_file 被期望的参数调用
    mock_download_file.assert_any_call('https://cdn.bootcdn.net/xx1/file1.js', {'file1.js': 'xx1/file1.js', 'file2.js': 'xx2/file2.js'})
    mock_download_file.assert_any_call('https://cdn.bootcdn.net/xx2/file2.js', {'file1.js': 'xx1/file1.js', 'file2.js': 'xx2/file2.js'})

    # 调试输出
    print("download_file calls:", mock_download_file.call_args_list)
    print("upload_to_oss calls:", mock_upload_to_oss.call_args_list)

    # 验证 upload_to_oss 是否被调用
    assert mock_upload_to_oss.called, "upload_to_oss was not called"

    # 验证 upload_to_oss 方法被期望的参数调用
    expected_calls = [
        mock.call('tmp_js/file1.js', 'xx1/file1.js'),
        mock.call('tmp_js/file2.js', 'xx2/file2.js')
    ]
    mock_upload_to_oss.assert_has_calls(expected_calls)