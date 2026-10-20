{
  description = "Alfa Hack 2026 — news impact analytics";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs =
    { nixpkgs, ... }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              gopls
              gotools
              golangci-lint
              delve
              air
              watchexec

              nodejs_24
              pnpm

              python313
              uv
              ruff

              gnumake
              git
              jq
              curl
              openssl
              pkg-config
              gcc

              postgresql_16
              redis
              minio-client
            ];

            shellHook = ''
              export GOTOOLCHAIN=local
              export UV_PYTHON=python3.13
              export UV_PYTHON_DOWNLOADS=never
              export UV_LINK_MODE=copy
              export PATH="$PWD/scripts:$PATH"

              echo "Alfa Hack dev shell"
              echo "  make demo   # Docker-only full stack"
              echo "  make dev    # native services + Docker infrastructure"

              command -v docker >/dev/null || \
                echo "warning: docker not found; 'make demo' will not work"
            '';
          };
        }
      );

      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt);
    };
}
