import React from "react";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { act } from "react"; // <-- use react's act
import App from "../App";
import axios from "axios";

// Mock window.location to prevent router context breakage
beforeAll(() => {
  // Only mock the pathname property, not the whole location object
  Object.defineProperty(window, "location", {
    configurable: true,
    value: {
      ...window.location,
      assign: jest.fn(),
      replace: jest.fn(),
      reload: jest.fn(),
      // Add all required properties for React Router
      href: "http://localhost/",
      origin: "http://localhost",
      protocol: "http:",
      host: "localhost",
      hostname: "localhost",
      port: "",
      pathname: "/",
      search: "",
      hash: "",
    },
  });
});

jest.mock("axios");

jest.mock("../components/profile/Profile", () => () => <div>Profile Component</div>);
jest.mock("../components/request/RequestTabPane", () => () => <div>RequestTabPane Component</div>);
jest.mock("../components/login/Login", () => () => <div>Login Component</div>);
jest.mock("../components/approve/ApproveTabPane", () => () => <div>ApproveTabPane Component</div>);
jest.mock("../components/history/HistoryTabPane", () => () => <div>HistoryTabPane Component</div>);
jest.mock("../components/admin/AdminTabPane", () => () => <div>AdminTabPane Component</div>);
jest.mock("../components/footer/Footer", () => () => <div>Footer Component</div>);
jest.mock("react-spinners", () => ({
  SyncLoader: () => <div>Loading Spinner</div>,
}));
jest.mock("../config/config", () => ({
  apiBaseUrl: "",
}));

(axios.get as jest.Mock).mockResolvedValue({ data: { sha: "testsha" } });
(axios.post as jest.Mock).mockResolvedValue({ data: {} });

describe("App", () => {
  it("renders without crashing", async () => {
    await act(async () => {
      render(
        <MemoryRouter initialEntries={["/"]}>
          <App />
        </MemoryRouter>
      );
    });
    expect(
      screen.getByText(/Profile Component|Login Component|RequestTabPane Component/)
    ).toBeInTheDocument();
  });
});