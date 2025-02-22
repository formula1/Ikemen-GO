import "./go_wasm_exec.js"

const IKEMEN_GO_TEMP_GAME_ID = "IKEMEN_GO_TEMP_GAME_ID"

const GO_WASM_PREP = Promise.resolve().then(async ()=>{
  console.log("Game Binding Start");
  if(!WebAssembly){
    throw new Error("WebAssembly is not available on your browser");
  }
  if (!WebAssembly.instantiateStreaming) {
    WebAssembly.instantiateStreaming = async (resp, importObject) => {
      const source = await (await resp).arrayBuffer();
      return await WebAssembly.instantiate(source, importObject);
    };
  }

  const response = await fetch("main.wasm");
  if(!response.ok){
    throw new Error("Failed to fetch main.wasm");
  }
  return response.arrayBuffer();
});

export async function setupGOInstance(gameId: string){
  const wasm = await GO_WASM_PREP
  const go = new Go();
  go.env["JS_INSTANCE"] = gameId
  const result = await WebAssembly.instantiate(wasm, go.importObject)
  go.run(result.instance);
}
