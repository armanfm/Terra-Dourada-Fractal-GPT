// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import {FunctionsClient} from "https://raw.githubusercontent.com/smartcontractkit/chainlink/v2.13.0/contracts/src/v0.8/functions/v1_0_0/FunctionsClient.sol";
import {FunctionsRequest} from "https://raw.githubusercontent.com/smartcontractkit/chainlink/v2.13.0/contracts/src/v0.8/functions/v1_0_0/libraries/FunctionsRequest.sol";

contract MarcasComChainlink is FunctionsClient {
    using FunctionsRequest for FunctionsRequest.Request;

    bytes32 public donId;
    uint64 public subscriptionId;
    uint32 public gasLimit = 300000;

    mapping(bytes32 => address) public requestDono;
    mapping(bytes32 => string) public requestNome;
    mapping(bytes32 => uint8) public requestCat;
    mapping(bytes32 => uint8) public requestScore;

    struct Marca {
        string nome;
        uint8 categoria;
        address dono;
        uint256 registradaEm;
        bool ativa;
        uint8 scoreRegistro;
    }
    mapping(bytes32 => Marca) public marcas;
    mapping(uint8 => bytes32[]) public hashsPorCategoria;
    mapping(address => bytes32[]) public marcasDoDono;
    uint256 public taxaRegistro = 0.001 ether;
    uint8 public limiteScore = 70;
    address public owner;

    event AnaliseeSolicitada(bytes32 indexed requestId, string nome, uint8 categoria);
    event MarcaRegistrada(bytes32 indexed id, string nome, uint8 categoria, address dono, uint256 timestamp, uint8 score);
    event MarcaRejeitada(bytes32 indexed requestId, string nome, string motivo);

    string public sourceCode = 
        "const nome = args[0];"
        "const bestMatch = args[1];"
        "const score = args[2];"
        "const seg = args[3];"
        "const apiKey = secrets.geminiKey;"
        "const prompt = `DECISION: APPROVED | REJECTED\\nINPUT: REQUESTED: ${nome} BEST_MATCH: ${bestMatch} SCORE: ${score}% SEGMENT: ${seg}\\nRespond ONLY: APPROVED or REJECTED`;"
        "const res = await Functions.makeHttpRequest({"
        "  url: `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=${apiKey}`,"
        "  method: 'POST',"
        "  data: { contents: [{ parts: [{ text: prompt }] }] }"
        "});"
        "const text = res.data.candidates[0].content.parts[0].text || '';"
        "return Functions.encodeUint256(text.includes('APPROVED') ? 1 : 0);";

    constructor(
        address router,
        bytes32 _donId,
        uint64 _subscriptionId
    ) FunctionsClient(router) {
        donId = _donId;
        subscriptionId = _subscriptionId;
        owner = msg.sender;
    }

    function solicitarAnalise(
        string memory nome,
        uint8 categoria,
        uint8 score,
        string memory bestMatch,
        string memory segmento,
        bytes memory encryptedSecretsRef
    ) external payable {
        require(score < limiteScore, "Score muito alto: muito similar!");
        require(msg.value >= taxaRegistro, "Taxa insuficiente!");
        require(categoria >= 1 && categoria <= 7, "Categoria invalida!");
        require(bytes(nome).length > 0, "Nome vazio!");

        FunctionsRequest.Request memory req;
        req.initializeRequestForInlineJavaScript(sourceCode);

        string[] memory args = new string[](4);
        args[0] = nome;
        args[1] = bestMatch;
        args[2] = uint2str(score);
        args[3] = segmento;
        req.setArgs(args);

        // ✅ CORREÇÃO 1: usar addSecretsReference para passar bytes encriptados
        if (encryptedSecretsRef.length > 0) {
            req.addSecretsReference(encryptedSecretsRef);
        }

        bytes32 requestId = _sendRequest(
            req.encodeCBOR(),
            subscriptionId,
            gasLimit,
            donId
        );

        requestDono[requestId] = msg.sender;
        requestNome[requestId] = nome;
        requestCat[requestId] = categoria;
        requestScore[requestId] = score;

        emit AnaliseeSolicitada(requestId, nome, categoria);
    }

    function fulfillRequest(
        bytes32 requestId,
        bytes memory response,
        bytes memory err
    ) internal override {
        address dono = requestDono[requestId];
        string memory nome = requestNome[requestId];
        uint8 cat = requestCat[requestId];
        uint8 score = requestScore[requestId];

        // ✅ CORREÇÃO 2: limpa estado ANTES de qualquer transfer (Checks-Effects-Interactions)
        delete requestDono[requestId];
        delete requestNome[requestId];
        delete requestCat[requestId];
        delete requestScore[requestId];

        if (err.length > 0) {
            emit MarcaRejeitada(requestId, nome, "Erro no Chainlink");
            payable(dono).transfer(taxaRegistro);
            return;
        }

        uint256 approved = abi.decode(response, (uint256));

        if (approved == 1) {
            bytes32 id = keccak256(abi.encodePacked(_toLower(nome), cat));
            marcas[id] = Marca({
                nome: nome,
                categoria: cat,
                dono: dono,
                registradaEm: block.timestamp,
                ativa: true,
                scoreRegistro: score
            });
            hashsPorCategoria[cat].push(id);
            marcasDoDono[dono].push(id);
            emit MarcaRegistrada(id, nome, cat, dono, block.timestamp, score);
        } else {
            emit MarcaRejeitada(requestId, nome, "Rejeitado pela IA");
            payable(dono).transfer(taxaRegistro);
        }
    }

    function getNomesAtivos(uint8 categoria) public view returns (string[] memory) {
        bytes32[] storage hashes = hashsPorCategoria[categoria];
        uint256 count = 0;
        for (uint256 i = 0; i < hashes.length; i++) {
            if (marcas[hashes[i]].ativa) count++;
        }
        string[] memory nomes = new string[](count);
        uint256 idx = 0;
        for (uint256 i = 0; i < hashes.length; i++) {
            if (marcas[hashes[i]].ativa) {
                nomes[idx] = marcas[hashes[i]].nome;
                idx++;
            }
        }
        return nomes;
    }

    function verificar(string memory nome, uint8 categoria)
        public view returns (bool, address, uint256, uint8)
    {
        bytes32 id = keccak256(abi.encodePacked(_toLower(nome), categoria));
        Marca storage m = marcas[id];
        return (m.ativa, m.dono, m.registradaEm, m.scoreRegistro);
    }

    function _toLower(string memory str) internal pure returns (string memory) {
        bytes memory b = bytes(str);
        for (uint i = 0; i < b.length; i++) {
            if (b[i] >= 0x41 && b[i] <= 0x5A)
                b[i] = bytes1(uint8(b[i]) + 32);
        }
        return string(b);
    }

    function uint2str(uint256 v) internal pure returns (string memory) {
        if (v == 0) return "0";
        uint256 tmp = v; uint256 digits;
        while (tmp != 0) { digits++; tmp /= 10; }
        bytes memory buf = new bytes(digits);
        while (v != 0) { digits--; buf[digits] = bytes1(uint8(48 + v % 10)); v /= 10; }
        return string(buf);
    }

    function setTaxa(uint256 nova) public {
        require(msg.sender == owner);
        taxaRegistro = nova;
    }

    function sacar() public {
        require(msg.sender == owner);
        payable(owner).transfer(address(this).balance);
    }

    receive() external payable {}
}
