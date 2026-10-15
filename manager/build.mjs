import { copyFileSync } from "node:fs";
copyFileSync("src/index.html", "dist/index.html");
copyFileSync("src/style.css", "dist/assets/console/style.css");
