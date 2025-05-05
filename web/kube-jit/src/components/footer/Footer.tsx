// Add this just before the final `export default App;` in App.tsx

function Footer() {
    return (
        <footer style={{
            marginTop: "2rem",
            padding: "1rem 0",
            background: "#f8f9fa",
            color: "#888",
            fontSize: "0.95em",
            textAlign: "center"
        }}>
            <div>
                © {new Date().getFullYear()} Kube-JIT |{" "}
                <a href="https://github.com/samirtahir91/kube-jit" target="_blank" rel="noopener noreferrer">
                    GitHub
                </a>{" "}
                | Apache 2.0 Licensed
            </div>
        </footer>
    );
}

export default Footer;