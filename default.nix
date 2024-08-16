{ pkgs ? import <nixpkgs> {}, lib ? pkgs.lib, ... }:

pkgs.buildGoModule rec {
  name = "r2wars-web-${version}";
  version = "1.0.0";

  src = ./.;
	vendorHash = null;

  CGO_ENABLED=0;

  subPackages = [ "src" ];
  postInstall = ''
    mv $out/bin/src $out/bin/r2wars-web
  '';
  
  meta = {
    description = "A golang implementation of r2wars";
    homepage = "https://r2wa.rs";
    license = lib.licenses.mit;
    maintainers = with lib.maintainers; [ hanemile ];
  };
}
