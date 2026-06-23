{
  lib,
  buildGoApplication,
  version ? (
    if builtins.pathExists ../.git then
      builtins.substring 0 7 (lib.commitIdFromGitRepo ../.git)
    else
      "dev"
  ),
}:
buildGoApplication {
  inherit version;

  pname = "lazyjira";
  src = ./..;
  modules = ../gomod2nix.toml;
  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];
  subPackages = [ "cmd/lazyjira" ];
  meta = with lib; {
    description = "Terminal UI for Jira";
    homepage = "https://github.com/textfuel/lazyjira";
    license = licenses.mit;
    mainProgram = "lazyjira";
  };
}
