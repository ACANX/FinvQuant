"""按内容哈希只追加保存原始数据对象。"""

from __future__ import annotations

import hashlib
import os
from dataclasses import dataclass
from pathlib import Path, PurePosixPath


class RawObjectStoreError(ValueError):
    """原始对象路径或内容不满足只追加存储约束。"""


@dataclass(frozen=True, slots=True)
class RawObjectRefV1:
    """原始对象的可重放来源引用。"""

    contentHash: str
    sourceRelativePath: str
    byteCount: int


class RawObjectStoreV1:
    """以 SHA-256 命名对象，拒绝覆盖既有不同内容。"""

    def __init__(self, root: Path) -> None:
        self._root = root.resolve()

    def storeBytes(self, content: bytes, sourceRelativePath: str) -> RawObjectRefV1:
        """存储或复用完全相同字节，并返回来源追溯引用。"""
        if not isinstance(content, bytes):
            raise RawObjectStoreError("原始对象必须是 bytes")
        relativePath = _validateRelativePath(sourceRelativePath)
        contentHash = hashlib.sha256(content).hexdigest()
        objectPath = self._objectPath(contentHash)
        objectPath.parent.mkdir(parents=True, exist_ok=True)
        try:
            descriptor = os.open(objectPath, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o444)
        except FileExistsError:
            self._verifyExisting(objectPath, contentHash)
        else:
            with os.fdopen(descriptor, "wb") as output:
                output.write(content)
                output.flush()
                os.fsync(output.fileno())
        return RawObjectRefV1(contentHash, relativePath, len(content))

    def readBytes(self, contentHash: str) -> bytes:
        """读取并再次验证对象没有被篡改。"""
        objectPath = self._objectPath(contentHash)
        try:
            content = objectPath.read_bytes()
        except FileNotFoundError as error:
            raise RawObjectStoreError("原始对象不存在") from error
        if hashlib.sha256(content).hexdigest() != contentHash:
            raise RawObjectStoreError("原始对象内容哈希不匹配")
        return content

    def _objectPath(self, contentHash: str) -> Path:
        if len(contentHash) != 64 or any(character not in "0123456789abcdef" for character in contentHash):
            raise RawObjectStoreError("内容哈希必须为小写 SHA-256")
        return self._root / "sha256" / contentHash[:2] / contentHash

    @staticmethod
    def _verifyExisting(objectPath: Path, expectedHash: str) -> None:
        content = objectPath.read_bytes()
        if hashlib.sha256(content).hexdigest() != expectedHash:
            raise RawObjectStoreError("已存在对象被篡改，拒绝覆盖")


def _validateRelativePath(value: str) -> str:
    if not value or "\\" in value:
        raise RawObjectStoreError("来源路径必须为非空 POSIX 相对路径")
    path = PurePosixPath(value)
    if path.is_absolute() or ".." in path.parts or path.parts[0] == ".":
        raise RawObjectStoreError("来源路径不得包含绝对路径、当前目录或上级目录")
    return path.as_posix()
