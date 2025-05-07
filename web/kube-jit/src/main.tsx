import ReactDOM from "react-dom/client";
import { BrowserRouter as Router } from 'react-router-dom';
import App from "./App.tsx";
import "./index.css";
import 'bootstrap-icons/font/bootstrap-icons.css';

ReactDOM.createRoot(document.getElementById("root")!).render(
    <Router>
        <App />
    </Router>
);