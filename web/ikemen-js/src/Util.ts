
export const Util = {
  async getClipboardText(){
    try {
      return await navigator.clipboard.readText();
    } catch (error) {
      console.error("Failed to read clipboard contents: ", error);
      return "";
    }
  },
  getTimeStampSeconds(){
    return Date.now() / 1000;
  }
}
