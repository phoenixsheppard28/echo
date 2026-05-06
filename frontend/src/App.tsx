import { useState } from "react";
function App() {
  const [message, setMessage] = useState("");
  const handleClick = () => {
    setMessage("Hello, world!");
  };
  return (
    <div className="container">
      <h1>echo</h1>
      <button onClick={handleClick}>Button</button>
      <p>{message}</p>
    </div>
  );
}

export default App;
