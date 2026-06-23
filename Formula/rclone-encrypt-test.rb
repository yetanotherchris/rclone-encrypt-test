class RcloneEncryptTest < Formula
  desc "CLI to encrypt/decrypt files using the rclone crypt format"
  homepage "https://github.com/yetanotherchris/rclone-encrypt-test"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/yetanotherchris/rclone-encrypt-test/releases/download/v0.1.0/rclone-encrypt-test-darwin-arm64.tar.gz"
      sha256 "9c58a8c6935a3a0af80f668b04cfcaf75ec39b0418775932247a32fbe59f50b7"
    else
      url "https://github.com/yetanotherchris/rclone-encrypt-test/releases/download/v0.1.0/rclone-encrypt-test-darwin-amd64.tar.gz"
      sha256 "0b6b88664996c5cb2a2bed4be5bbd98be065f1754c4e881c4f0207b7b3c50598"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/yetanotherchris/rclone-encrypt-test/releases/download/v0.1.0/rclone-encrypt-test-linux-arm64.tar.gz"
      sha256 "41073bf67d17219d99797c09645b3ee1d24217fff54857ca5da71d8b3d7af1c0"
    else
      url "https://github.com/yetanotherchris/rclone-encrypt-test/releases/download/v0.1.0/rclone-encrypt-test-linux-amd64.tar.gz"
      sha256 "508d0772c1bebb62adfda07d27a5a76cad4dee31ab2a4648346eb8b5c6c53f07"
    end
  end

  def install
    bin.install "rclone-encrypt-test-darwin-arm64" => "rclone-encrypt-test" if OS.mac? && Hardware::CPU.arm?
    bin.install "rclone-encrypt-test-darwin-amd64" => "rclone-encrypt-test" if OS.mac? && !Hardware::CPU.arm?
    bin.install "rclone-encrypt-test-linux-arm64" => "rclone-encrypt-test" if OS.linux? && Hardware::CPU.arm?
    bin.install "rclone-encrypt-test-linux-amd64" => "rclone-encrypt-test" if OS.linux? && !Hardware::CPU.arm?
  end

  test do
    assert_match "rclone-encrypt-test #{version}", shell_output("#{bin}/rclone-encrypt-test --version")
  end
end