{pkgs, ...}: {
  # https://devenv.sh/packages/
  packages = with pkgs; [
    golangci-lint
    gopls
    sqlite # inspecting the stats DB by hand
  ];

  # https://devenv.sh/languages/
  languages.go = {
    enable = true;
    version = "1.26.7";
    enableHardeningWorkaround = true;
  };

  env = {
    GOTOOLCHAIN = "local"; # prevent go from messing with go.mod regarding go/toolchain version
    hardeningDisable = ["all"]; # needed for any GOGCC stuff under nix
    GONOSUMDB = "g1tlab.1nnov8.de/*,rain.cloud/*";
    GOPROXY = "http://ryzerv.1nnov8.eu:3000,direct";
  };

  enterShell = ''
    echo -e "Now in \e[3m\e[32mdevenv\e[0m \e[36mdevShell\e[0m..."
    go version
    go env -w GOTOOLCHAIN=local
    go env -w GONOSUMDB="g1tlab.1nnov8.de/*,rain.cloud/*"
    if ping -c1 ryzerv.1nnov8.eu >/dev/null 2>&1; then
       go env -w GOPROXY="http://ryzerv.1nnov8.eu:3000,direct"
    fi;
  '';
}
