from clyde.scanner import scan_repo


def test_scan_repo_skips_likely_secret(tmp_path) -> None:
    (tmp_path / "ok.py").write_text("print('safe')\n")
    fake_key = "abc" + "def" + "ghi" + "jkl" + "mno" + "pqr" + "stu" + "vwx" + "yz123456"
    (tmp_path / "secret.env").write_text(f"API_KEY='{fake_key}'\n")

    result = scan_repo(tmp_path)

    assert [item.rel_path for item in result.files] == ["ok.py"]
    assert result.skips[0].rel_path == "secret.env"
    assert result.skips[0].reason == "possible secret material"


def test_scan_repo_respects_exclude(tmp_path) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")
    (tmp_path / "notes.md").write_text("# Notes\n")

    result = scan_repo(tmp_path, exclude=["*.md"])

    assert [item.rel_path for item in result.files] == ["app.py"]
