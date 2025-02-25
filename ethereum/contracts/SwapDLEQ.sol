// SPDX-License-Identifier: MIT

pragma solidity ^0.8.5;

// import "./Ed25519.sol";
// import "./Ed25519_alt.sol";
import "./EC.sol";

contract SwapDLEQ {
    // EC library
    EC immutable ec;

    // contract creator, Alice
    address payable immutable owner;

    // the expected public key derived from the secret `s_b`.
    // this public key is a point on the ed25519 curve
    uint256 public immutable pubKeyClaimX;
    uint256 public immutable pubKeyClaimY;

    // the expected public key derived from the secret `s_a`.
    // this public key is a point on the ed25519 curve
    uint256 public immutable pubKeyRefundX;
    uint256 public immutable pubKeyRefundY;

    // time period from contract creation
    // during which Alice can call either set_ready or refund
    uint256 public immutable timeout_0;

    // time period from the moment Alice calls `set_ready`
    // during which Bob can claim. After this, Alice can refund again
    uint256 public timeout_1;

    // Alice sets ready to true when she sees the funds locked on the other chain.
    // this prevents Bob from withdrawing funds without locking funds on the other chain first
    bool isReady = false;

    event Constructed(uint256 x, uint256 y);
    event IsReady(bool b);
    event Claimed(uint256 s);
    event Refunded(uint256 s);

    constructor(
        uint256 _pubKeyClaimX,
        uint256 _pubKeyClaimY,
        uint256 _pubKeyRefundX,
        uint256 _pubKeyRefundY
    ) payable {
        owner = payable(msg.sender);
        pubKeyClaimX = _pubKeyClaimX;
        pubKeyClaimY = _pubKeyClaimY;
        pubKeyRefundX = _pubKeyRefundX;
        pubKeyRefundY = _pubKeyRefundY;
        timeout_0 = block.timestamp + 1 days;
        ec = new EC();
        emit Constructed(_pubKeyRefundX, _pubKeyRefundY);
    }

    // Alice must call set_ready() within t_0 once she verifies the XMR has been locked
    function set_ready() external {
        require(!isReady && msg.sender == owner && block.timestamp < timeout_0);
        isReady = true;
        timeout_1 = block.timestamp + 1 days;
        emit IsReady(true);
    }

    // Bob can claim if:
    // - Alice doesn't call set_ready or refund within t_0, or
    // - Alice calls ready within t_0, in which case Bob can call claim until t_1
    function claim(uint256 _s) external {
        if (isReady == true) {
            require(block.timestamp < timeout_1, "Too late to claim!");
        } else {
            require(
                block.timestamp >= timeout_0,
                "'isReady == false' cannot claim yet!"
            );
        }

        require(
            ec.publicKeyVerify(_s, pubKeyClaimX, pubKeyClaimY),
            "provided secret does not match the expected pubKey"
        );
        emit Claimed(_s);

        // send eth to caller (Bob)
        selfdestruct(payable(msg.sender));
    }

    // Alice can claim a refund:
    // - Until t_0 unless she calls set_ready
    // - After t_1, if she called set_ready
    function refund(uint256 _s) external {
        if (isReady == true) {
            require(
                block.timestamp >= timeout_1,
                "It's Bob's turn now, please wait!"
            );
        } else {
            require(block.timestamp < timeout_0, "Missed your chance!");
        }

        require(
            ec.publicKeyVerify(_s, pubKeyRefundX, pubKeyRefundY),
            "provided secret does not match the expected pubKey"
        );
        emit Refunded(_s);

        // send eth back to owner==caller (Alice)
        selfdestruct(owner);
    }
}
